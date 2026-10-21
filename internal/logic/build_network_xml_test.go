package logic_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type networkXML struct {
	XMLName          xml.Name `xml:"NetworkParameter"`
	TargetMAC        string   `xml:"TargetMAC"`
	DHCP             string   `xml:"DHCP"`
	IPv4Address      string   `xml:"IPv4Address"`
	SubnetMask       string   `xml:"SubnetMask"`
	DefaultGateway   string   `xml:"DefaultGateway"`
	SecurityResponse string   `xml:"SecurityResponse"`
	Password         string   `xml:"Password"`
}

func TestBuildNetworkXMLSerializesDHCPRequestWithChallengeResponse(t *testing.T) {
	payload, err := logic.BuildNetworkXML(
		net.HardwareAddr{0, 17, 34, 51, 68, 85},
		logic.NetworkConfig{UseDHCP: true},
		"12345678",
		"admin",
	)
	if err != nil {
		t.Fatalf("BuildNetworkXML returned error: %v", err)
	}
	if !bytes.HasPrefix(payload, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("payload lacks UTF-8 XML declaration: %q", payload)
	}
	var document networkXML
	if err := xml.Unmarshal(payload, &document); err != nil {
		t.Fatalf("payload is not XML: %v", err)
	}
	if document.XMLName.Local != "NetworkParameter" || document.TargetMAC != "00:11:22:33:44:55" {
		t.Fatalf("unexpected request identity: %#v", document)
	}
	if document.DHCP != "true" || document.IPv4Address != "" || document.SubnetMask != "" || document.DefaultGateway != "" {
		t.Fatalf("unexpected DHCP fields: %#v", document)
	}
	if document.SecurityResponse != "3c8872c094682f4c3fcdffdab80a8d351dbeee9b31a29e6ea092b51cf473e732" {
		t.Fatalf("SecurityResponse = %q", document.SecurityResponse)
	}
	if document.Password != "" {
		t.Fatal("network XML must not contain plaintext Password")
	}
}

func TestBuildNetworkXMLSerializesStaticIPv4Request(t *testing.T) {
	payload, err := logic.BuildNetworkXML(
		net.HardwareAddr{10, 11, 12, 13, 14, 15},
		logic.NetworkConfig{
			Address: netip.MustParsePrefix("192.0.2.20/24"),
			Gateway: netip.MustParseAddr("192.0.2.1"),
		},
		"challenge&token",
		"Pass&word1!",
	)
	if err != nil {
		t.Fatalf("BuildNetworkXML returned error: %v", err)
	}
	var document networkXML
	if err := xml.Unmarshal(payload, &document); err != nil {
		t.Fatalf("payload is not XML: %v", err)
	}
	if document.DHCP != "false" || document.IPv4Address != "192.0.2.20" ||
		document.SubnetMask != "255.255.255.0" || document.DefaultGateway != "192.0.2.1" {
		t.Fatalf("unexpected static fields: %#v", document)
	}
	if document.SecurityResponse != "7f77d5214ef8164be1fd5099aaa6fb1dcf9d78c42f42f2faec03db3c582c79f5" {
		t.Fatalf("SecurityResponse = %q", document.SecurityResponse)
	}
}

func TestBuildNetworkXMLRejectsInvalidInputsWithEmptyPayload(t *testing.T) {
	validMAC := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	static := logic.NetworkConfig{
		Address: netip.MustParsePrefix("192.0.2.20/24"),
		Gateway: netip.MustParseAddr("192.0.2.1"),
	}
	testCases := []struct {
		name   string
		mac    net.HardwareAddr
		config logic.NetworkConfig
		chal   string
		pass   string
	}{
		{name: "empty MAC", config: static, chal: "challenge", pass: "password"},
		{name: "non EUI-48 MAC", mac: net.HardwareAddr{1, 2, 3}, config: static, chal: "challenge", pass: "password"},
		{name: "DHCP with static address", mac: validMAC, config: logic.NetworkConfig{UseDHCP: true, Address: static.Address}, chal: "challenge", pass: "password"},
		{name: "DHCP with gateway", mac: validMAC, config: logic.NetworkConfig{UseDHCP: true, Gateway: static.Gateway}, chal: "challenge", pass: "password"},
		{name: "static without address", mac: validMAC, config: logic.NetworkConfig{Gateway: static.Gateway}, chal: "challenge", pass: "password"},
		{name: "static without gateway", mac: validMAC, config: logic.NetworkConfig{Address: static.Address}, chal: "challenge", pass: "password"},
		{name: "IPv6 address", mac: validMAC, config: logic.NetworkConfig{Address: netip.MustParsePrefix("2001:db8::20/64"), Gateway: netip.MustParseAddr("192.0.2.1")}, chal: "challenge", pass: "password"},
		{name: "missing challenge", mac: validMAC, config: static, pass: "password"},
		{name: "missing password", mac: validMAC, config: static, chal: "challenge"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := logic.BuildNetworkXML(testCase.mac, testCase.config, testCase.chal, testCase.pass)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if len(payload) != 0 {
				t.Fatalf("payload = %q, want empty payload", payload)
			}
			var operationErr *logic.DeviceOperationError
			if !errors.As(err, &operationErr) || operationErr.Kind != logic.OperationInvalidInput {
				t.Fatalf("error = %T %v, want invalid-input DeviceOperationError", err, err)
			}
		})
	}
}

func TestBuildNetworkXMLDoesNotDiscloseSecurityInputsOrUsePlaintextFallback(t *testing.T) {
	const password = "network-password-canary"
	const challenge = "network-challenge-canary"

	payload, err := logic.BuildNetworkXML(
		net.HardwareAddr{0, 17, 34, 51, 68, 85},
		logic.NetworkConfig{UseDHCP: true},
		"",
		password,
	)
	if err == nil {
		t.Fatal("missing challenge must fail closed")
	}
	if len(payload) != 0 || strings.Contains(err.Error(), password) || strings.Contains(err.Error(), challenge) {
		t.Fatalf("unsafe failure result: payload=%q error=%v", payload, err)
	}
	if strings.Contains(string(payload), password) {
		t.Fatal("plaintext password must never be serialized")
	}
}
