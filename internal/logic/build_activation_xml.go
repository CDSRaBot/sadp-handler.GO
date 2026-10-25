package logic

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

type ActivationRequest struct {
	MAC      net.HardwareAddr
	Password string
	Email    string
}

type NetworkConfig struct {
	Address netip.Prefix
	Gateway netip.Addr
	UseDHCP bool
}

func BuildActivationXML(request ActivationRequest) ([]byte, error) {
	if err := validateMAC(request.MAC); err != nil {
		return nil, invalidOperation("MAC", err)
	}
	if err := ValidatePassword(request.Password); err != nil {
		return nil, invalidOperation("password", err)
	}

	document := activationXML{
		TargetMAC: strings.ToUpper(request.MAC.String()),
		Password:  request.Password,
	}
	if request.Email != "" {
		document.OptionData = &activationOptionData{SecurityEmail: request.Email}
	}
	return marshalXML(document)
}

func BuildNetworkXML(mac net.HardwareAddr, config NetworkConfig, challenge string, password string) ([]byte, error) {
	if err := validateMAC(mac); err != nil {
		return nil, invalidOperation("MAC", err)
	}
	if err := validateNetworkConfig(config); err != nil {
		return nil, invalidOperation("network configuration", err)
	}
	response, err := CalculateHash(password, challenge)
	if err != nil {
		return nil, invalidOperation("security response", err)
	}

	document := networkXML{
		TargetMAC:        strings.ToUpper(mac.String()),
		DHCP:             fmt.Sprintf("%t", config.UseDHCP),
		SecurityResponse: response,
	}
	if !config.UseDHCP {
		ones := config.Address.Bits()
		document.IPv4Address = config.Address.Addr().String()
		document.SubnetMask = net.IP(net.CIDRMask(ones, 32)).String()
		document.DefaultGateway = config.Gateway.String()
	}
	return marshalXML(document)
}

type activationXML struct {
	XMLName    xml.Name              `xml:"Activate"`
	TargetMAC  string                `xml:"TargetMAC"`
	Password   string                `xml:"Password"`
	OptionData *activationOptionData `xml:"OptionData,omitempty"`
}

type activationOptionData struct {
	SecurityEmail string `xml:"SecurityEmail"`
}

type networkXML struct {
	XMLName          xml.Name `xml:"NetworkParameter"`
	TargetMAC        string   `xml:"TargetMAC"`
	DHCP             string   `xml:"DHCP"`
	IPv4Address      string   `xml:"IPv4Address,omitempty"`
	SubnetMask       string   `xml:"SubnetMask,omitempty"`
	DefaultGateway   string   `xml:"DefaultGateway,omitempty"`
	SecurityResponse string   `xml:"SecurityResponse"`
}

func validateMAC(mac net.HardwareAddr) error {
	if len(mac) != 6 {
		return errors.New("MAC must be an EUI-48 address")
	}
	return nil
}

func validateNetworkConfig(config NetworkConfig) error {
	if config.UseDHCP {
		if config.Address.IsValid() || config.Gateway.IsValid() {
			return errors.New("DHCP cannot include static address or gateway")
		}
		return nil
	}
	if !config.Address.IsValid() || !config.Address.Addr().Is4() {
		return errors.New("static address must be IPv4")
	}
	if config.Address.Bits() < 0 {
		return errors.New("static address must have an IPv4 prefix")
	}
	if !config.Gateway.IsValid() || !config.Gateway.Is4() || config.Gateway.IsUnspecified() {
		return errors.New("static gateway must be IPv4")
	}
	return nil
}

func invalidOperation(field string, err error) error {
	return &DeviceOperationError{
		Kind: OperationInvalidInput,
		Err:  fmt.Errorf("%s is invalid: %w", field, err),
	}
}

func marshalXML(value any) ([]byte, error) {
	data, err := xml.Marshal(value)
	if err != nil {
		return nil, &DeviceOperationError{Kind: OperationInvalidInput, Err: fmt.Errorf("serialize XML: %w", err)}
	}
	return append([]byte(xml.Header), data...), nil
}
