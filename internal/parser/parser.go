package parser

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
)

// Device is the validated device information extracted from an advertisement.
type Device struct {
	MAC             net.HardwareAddr
	UUID            string
	Types           string
	IPv4            netip.Addr
	IPv4Assigned    bool
	IPv4Mask        netip.Addr
	IPv4Gateway     netip.Addr
	DHCP            bool
	Activated       bool
	Model           string
	SerialNumber    string
	SoftwareVersion string
	Sender          net.Addr
}

type NetworkAddressState string

const (
	NetworkAddressAssigned NetworkAddressState = "assigned"
	NetworkAddressPending  NetworkAddressState = "pending"
	NetworkAddressUnknown  NetworkAddressState = "unknown"
)

func (d Device) AddressState() NetworkAddressState {
	if !d.IPv4.IsValid() || (d.DHCP && d.IPv4.IsUnspecified()) {
		return NetworkAddressPending
	}
	if d.IPv4Assigned {
		return NetworkAddressAssigned
	}
	return NetworkAddressUnknown
}

// ParseErrorKind identifies the stage at which advertisement parsing failed.
type ParseErrorKind string

const (
	ParseEmptyDatagram     ParseErrorKind = "empty_datagram"
	ParseOversizedDatagram ParseErrorKind = "oversized_datagram"
	ParseMalformedXML      ParseErrorKind = "malformed_xml"
	ParseUnsupported       ParseErrorKind = "unsupported_message"
	ParseMissingField      ParseErrorKind = "missing_field"
	ParseInvalidField      ParseErrorKind = "invalid_field"
)

// ParseError describes a safe, classified advertisement parsing failure.
type ParseError struct {
	Kind  ParseErrorKind
	Field string
	Err   error
}

func (e *ParseError) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}
	return string(e.Kind) + ": " + e.Err.Error()
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func ParseAdvertisement(data []byte, sender net.Addr, maxDatagramSize int) (Device, error) {
	if len(data) == 0 {
		return Device{}, parseError(ParseEmptyDatagram, "", errors.New("datagram is empty"))
	}
	if maxDatagramSize < 1 || maxDatagramSize > 65507 {
		return Device{}, parseError(ParseInvalidField, "maxDatagramSize", errors.New("size must be between 1 and 65507"))
	}
	if len(data) > maxDatagramSize {
		return Device{}, parseError(ParseOversizedDatagram, "", errors.New("datagram exceeds configured limit"))
	}
	if sender == nil {
		return Device{}, parseError(ParseInvalidField, "sender", errors.New("sender is nil"))
	}

	fields, err := decodeAdvertisement(data)
	if err != nil {
		return Device{}, err
	}
	if fields.root == "ProbeMatch" && !supportedType(fields.types) {
		return Device{}, parseError(ParseUnsupported, "", errors.New("unsupported message type"))
	}
	if fields.root == "Hello" && fields.types != "hello" {
		return Device{}, parseError(ParseUnsupported, "", errors.New("unsupported message type"))
	}
	if strings.ContainsRune(fields.model, 0) {
		return Device{}, parseError(ParseInvalidField, "Model", errors.New("conflicting model aliases"))
	}
	if strings.ContainsRune(fields.serial, 0) {
		return Device{}, parseError(ParseInvalidField, "SerialNumber", errors.New("conflicting serial aliases"))
	}

	if !fields.macPresent {
		return Device{}, parseError(ParseMissingField, "MAC", errors.New("required field is missing"))
	}
	if fields.dhcpPresent && fields.dhcpValue != "" && fields.dhcpValue != "true" && fields.dhcpValue != "false" {
		return Device{}, parseError(ParseInvalidField, "DHCP", errors.New("value must be true or false"))
	}
	mac, err := NormalizeMAC(fields.mac)
	if err != nil {
		return Device{}, parseError(ParseInvalidField, "MAC", err)
	}
	if !fields.ipv4Present {
		return Device{}, parseError(ParseMissingField, "IPv4Address", errors.New("required field is missing"))
	}
	ipv4, err := netip.ParseAddr(fields.ipv4)
	if err != nil || !ipv4.Is4() || (ipv4.IsUnspecified() && !fields.dhcp) {
		if err == nil {
			err = errors.New("value is not a usable IPv4 address")
		}
		return Device{}, parseError(ParseInvalidField, "IPv4Address", err)
	}
	mask, err := parseOptionalIPv4(fields.ipv4Mask, "SubnetMask")
	if err != nil {
		return Device{}, err
	}
	gateway, err := parseOptionalIPv4(fields.ipv4Gateway, "DefaultGateway")
	if err != nil {
		return Device{}, err
	}

	activated := false
	if fields.activated != "" {
		switch strings.ToLower(fields.activated) {
		case "true":
			activated = true
		case "false":
		default:
			return Device{}, parseError(ParseInvalidField, "Activated", errors.New("value must be true or false"))
		}
	}

	return Device{
		MAC:             mac,
		UUID:            fields.uuid,
		Types:           fields.types,
		IPv4:            ipv4,
		IPv4Assigned:    !ipv4.IsUnspecified(),
		IPv4Mask:        mask,
		IPv4Gateway:     gateway,
		DHCP:            fields.dhcp,
		Activated:       activated,
		Model:           fields.model,
		SerialNumber:    fields.serial,
		SoftwareVersion: fields.softwareVersion,
		Sender:          sender,
	}, nil
}

func supportedType(value string) bool {
	switch value {
	case "inquiry", "update", "activate", "exchangecode", "getsecurityquestion", "setsecurityquestion", "SetHCPlatform":
		return true
	default:
		return false
	}
}

func parseOptionalIPv4(value, field string) (netip.Addr, error) {
	if value == "" {
		return netip.Addr{}, nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is4() {
		if err == nil {
			err = errors.New("value is not an IPv4 address")
		}
		return netip.Addr{}, parseError(ParseInvalidField, field, err)
	}
	return address, nil
}

func NormalizeMAC(value string) (net.HardwareAddr, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid hardware address: %w", err)
	}
	if len(mac) != 6 {
		return nil, errors.New("hardware address is not EUI-48")
	}
	return append(net.HardwareAddr(nil), mac...), nil
}

type advertisementFields struct {
	root            string
	types           string
	uuid            string
	mac             string
	ipv4            string
	ipv4Mask        string
	ipv4Gateway     string
	dhcp            bool
	dhcpValue       string
	activated       string
	model           string
	serial          string
	softwareVersion string
	macPresent      bool
	ipv4Present     bool
	dhcpPresent     bool
}

func decodeAdvertisement(data []byte) (advertisementFields, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	fields := advertisementFields{softwareVersion: "Unknown"}
	var rootDepth int
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if rootDepth == 0 {
				return fields, parseError(ParseMalformedXML, "", errors.New("document has no root element"))
			}
			return fields, parseError(ParseMalformedXML, "", errors.New("unexpected end of document"))
		}
		if err != nil {
			return fields, parseError(ParseMalformedXML, "", err)
		}
		switch value := token.(type) {
		case xml.ProcInst:
			if rootDepth != 0 || strings.ToLower(string(value.Target)) != "xml" {
				return fields, parseError(ParseMalformedXML, "", errors.New("unsupported processing instruction"))
			}
		case xml.Directive:
			return fields, parseError(ParseMalformedXML, "", errors.New("XML directives are not supported"))
		case xml.StartElement:
			if rootDepth != 0 {
				return fields, parseError(ParseMalformedXML, "", errors.New("nested root element"))
			}
			fields.root = value.Name.Local
			if fields.root != "ProbeMatch" && fields.root != "Hello" {
				return fields, parseError(ParseUnsupported, "", errors.New("unsupported root element"))
			}
			rootDepth = 1
			if err := decodeChildren(decoder, &fields, value); err != nil {
				return fields, err
			}
			if err := decodeTrailing(decoder); err != nil {
				return fields, err
			}
			return fields, nil
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return fields, parseError(ParseMalformedXML, "", errors.New("text outside root element"))
			}
		}
	}
}

func decodeTrailing(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return parseError(ParseMalformedXML, "", err)
		}
		switch value := token.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return parseError(ParseMalformedXML, "", errors.New("trailing XML content"))
			}
		case xml.Comment:
		default:
			return parseError(ParseMalformedXML, "", errors.New("trailing XML content"))
		}
	}
}

func decodeChildren(decoder *xml.Decoder, fields *advertisementFields, root xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return parseError(ParseMalformedXML, "", errors.New("unexpected end of document"))
			}
			return parseError(ParseMalformedXML, "", err)
		}
		switch value := token.(type) {
		case xml.EndElement:
			if value.Name == root.Name {
				return nil
			}
			return parseError(ParseMalformedXML, "", errors.New("unexpected closing element"))
		case xml.StartElement:
			text, err := readElementText(decoder, value)
			if err != nil {
				return err
			}
			assignField(fields, value.Name.Local, text)
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return parseError(ParseMalformedXML, "", errors.New("text outside direct child"))
			}
		case xml.Directive:
			return parseError(ParseMalformedXML, "", errors.New("XML directives are not supported"))
		case xml.ProcInst:
			return parseError(ParseMalformedXML, "", errors.New("processing instruction is not supported"))
		}
	}
}

func readElementText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", parseError(ParseMalformedXML, "", err)
		}
		switch value := token.(type) {
		case xml.CharData:
			text.Write([]byte(value))
		case xml.EndElement:
			if value.Name == start.Name {
				return strings.TrimSpace(text.String()), nil
			}
			return "", parseError(ParseMalformedXML, "", errors.New("unexpected closing element"))
		case xml.StartElement:
			return "", parseError(ParseMalformedXML, "", errors.New("nested elements are not supported"))
		case xml.Directive, xml.ProcInst:
			return "", parseError(ParseMalformedXML, "", errors.New("XML instructions are not supported"))
		}
	}
}

func assignField(fields *advertisementFields, name, value string) {
	switch name {
	case "Types":
		fields.types = value
	case "Uuid", "UUID":
		fields.uuid = value
	case "MAC":
		fields.mac = value
		fields.macPresent = true
	case "IPv4Address":
		fields.ipv4 = value
		fields.ipv4Present = true
	case "SubnetMask":
		fields.ipv4Mask = value
	case "DefaultGateway":
		fields.ipv4Gateway = value
	case "DHCP":
		fields.dhcpPresent = true
		fields.dhcpValue = strings.ToLower(value)
		switch strings.ToLower(value) {
		case "true":
			fields.dhcp = true
		case "false", "":
		default:
			fields.dhcp = false
		}
	case "Activated":
		fields.activated = value
	case "DeviceModel":
		fields.model = mergeAlias(fields.model, value)
	case "DeviceDescription":
		fields.model = mergeAlias(fields.model, value)
	case "SerialNumber":
		fields.serial = mergeAlias(fields.serial, value)
	case "DeviceSN":
		fields.serial = mergeAlias(fields.serial, value)
	case "SoftwareVersion":
		fields.softwareVersion = value
	}
}

func mergeAlias(existing, candidate string) string {
	if existing == "" {
		return candidate
	}
	if candidate == "" || existing == candidate {
		return existing
	}
	return existing + "\x00" + candidate
}

func parseError(kind ParseErrorKind, field string, err error) error {
	return &ParseError{Kind: kind, Field: field, Err: err}
}
