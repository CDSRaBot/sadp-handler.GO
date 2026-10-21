package parser

import (
	"errors"
	"net"
	"net/netip"
)

// Device is the validated device information extracted from an advertisement.
type Device struct {
	MAC             net.HardwareAddr
	IPv4            netip.Addr
	Activated       bool
	Model           string
	SerialNumber    string
	SoftwareVersion string
	Sender          net.Addr
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

// ParseAdvertisement is the Phase 4 stub for the M2 parser.
func ParseAdvertisement(data []byte, sender net.Addr, maxDatagramSize int) (Device, error) {
	return Device{}, errors.New("not implemented")
}

// NormalizeMAC is the Phase 4 stub for MAC address normalization.
func NormalizeMAC(value string) (net.HardwareAddr, error) {
	return nil, errors.New("not implemented")
}
