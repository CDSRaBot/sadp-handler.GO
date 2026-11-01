package logic

import (
	"errors"
	"net"
	"strings"
)

// SADPWireProfile identifies a device-family wire contract. The legacy profile
// is retained for compatibility; the salted profile requires an externally
// verified codec because SADP key derivation and IV rules are not established.
type SADPWireProfile string

const (
	SADPWireProfileLegacy    SADPWireProfile = "legacy-sha256"
	SADPWireProfileSaltedAES SADPWireProfile = "salted-aes"
)

type SADPSensitiveFieldEncoder func(value, salt string) (string, error)

type SADPWireOptions struct {
	Profile SADPWireProfile
	Encode  SADPSensitiveFieldEncoder
}

func BuildNetworkXMLWithProfile(mac net.HardwareAddr, config NetworkConfig, challenge, password string, options SADPWireOptions) ([]byte, error) {
	if options.Profile == "" || options.Profile == SADPWireProfileLegacy {
		return BuildNetworkXML(mac, config, challenge, password)
	}
	if options.Profile != SADPWireProfileSaltedAES || options.Encode == nil {
		return nil, invalidOperation("wire profile", errors.New("salted profile requires a verified sensitive-field encoder"))
	}
	if err := validateMAC(mac); err != nil {
		return nil, invalidOperation("MAC", err)
	}
	if err := validateNetworkConfig(config); err != nil {
		return nil, invalidOperation("network configuration", err)
	}
	if password == "" || challenge == "" {
		return nil, invalidOperation("security inputs", errors.New("password and challenge are required"))
	}
	encoded, err := options.Encode(password, challenge)
	if err != nil {
		return nil, invalidOperation("salted password", err)
	}
	if encoded == "" {
		return nil, invalidOperation("salted password", errors.New("encoder returned an empty value"))
	}
	document := networkXML{
		TargetMAC:        strings.ToUpper(mac.String()),
		DHCP:             boolString(config.UseDHCP),
		SecurityResponse: encoded,
	}
	if !config.UseDHCP {
		ones := config.Address.Bits()
		document.IPv4Address = config.Address.Addr().String()
		document.SubnetMask = net.IP(net.CIDRMask(ones, 32)).String()
		document.DefaultGateway = config.Gateway.String()
	}
	return marshalXML(document)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
