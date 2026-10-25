package parser_test

import (
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

const (
	defaultLimit = 1500
	senderIP     = "192.0.2.10"
)

func TestNormalizeMAC(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "colon separated", value: "00:11:22:33:44:55", want: "00:11:22:33:44:55"},
		{name: "hyphen separated", value: "00-11-22-33-44-55", want: "00:11:22:33:44:55"},
		{name: "plain hex", value: "001122334455", want: "00:11:22:33:44:55"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parser.NormalizeMAC(testCase.value)
			if err != nil {
				t.Fatalf("NormalizeMAC returned error: %v", err)
			}
			if got.String() != testCase.want {
				t.Fatalf("MAC = %q, want %q", got, testCase.want)
			}
		})
	}

	for _, value := range []string{"", "not-a-mac", "00:11:22:33:44", "00:11:22:33:44:55:66:77"} {
		t.Run("reject "+value, func(t *testing.T) {
			if got, err := parser.NormalizeMAC(value); err == nil || got != nil {
				t.Fatalf("NormalizeMAC(%q) = %#v, %v; want error and nil", value, got, err)
			}
		})
	}
}

func TestParseAdvertisementAcceptsReferenceProbeMatch(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
	device, err := parser.ParseAdvertisement(referenceProbeMatch, sender, defaultLimit)
	if err != nil {
		t.Fatalf("ParseAdvertisement returned error: %v", err)
	}
	assertDevice(t, device, "00:11:22:33:44:55", "192.0.2.20", true, "DS-TEST-1", "SN-TEST-1", "V1.2.3", sender)
}

func TestParseAdvertisementAcceptsObservedAliasesAndHelloWithoutUUID(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
	for _, testCase := range []struct {
		name string
		data []byte
		want bool
	}{
		{name: "probe match aliases", data: observedProbeMatch, want: false},
		{name: "hello without uuid", data: helloWithoutUUID, want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			device, err := parser.ParseAdvertisement(testCase.data, sender, defaultLimit)
			if err != nil {
				t.Fatalf("ParseAdvertisement returned error: %v", err)
			}
			assertDevice(t, device, "aa:bb:cc:dd:ee:ff", "198.51.100.20", testCase.want, "Observed device", "OBS-SN-1", "Unknown", sender)
		})
	}
}

func TestParseAdvertisementValidatesLimitsAndDocumentShape(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
	testCases := []struct {
		name  string
		data  []byte
		limit int
		kind  parser.ParseErrorKind
		field string
	}{
		{name: "empty", data: nil, limit: defaultLimit, kind: parser.ParseEmptyDatagram},
		{name: "limit zero", data: referenceProbeMatch, limit: 0, kind: parser.ParseInvalidField, field: "maxDatagramSize"},
		{name: "limit too large", data: referenceProbeMatch, limit: 65508, kind: parser.ParseInvalidField, field: "maxDatagramSize"},
		{name: "oversized", data: referenceProbeMatch, limit: len(referenceProbeMatch) - 1, kind: parser.ParseOversizedDatagram},
		{name: "malformed xml", data: []byte("<ProbeMatch>"), limit: defaultLimit, kind: parser.ParseMalformedXML},
		{name: "trailing content", data: append(append([]byte{}, referenceProbeMatch...), []byte("<extra/>")...), limit: 2000, kind: parser.ParseMalformedXML},
		{name: "invalid utf8", data: []byte("<ProbeMatch>\xff</ProbeMatch>"), limit: defaultLimit, kind: parser.ParseMalformedXML},
		{name: "directive", data: []byte(`<!DOCTYPE ProbeMatch><ProbeMatch/>`), limit: defaultLimit, kind: parser.ParseMalformedXML},
		{name: "unsupported root", data: []byte(`<Other><Types>inquiry</Types></Other>`), limit: defaultLimit, kind: parser.ParseUnsupported},
		{name: "wrong type", data: []byte(`<Hello><Types>inquiry</Types></Hello>`), limit: defaultLimit, kind: parser.ParseUnsupported},
		{name: "missing mac", data: []byte(`<ProbeMatch><Types>inquiry</Types><IPv4Address>192.0.2.20</IPv4Address></ProbeMatch>`), limit: defaultLimit, kind: parser.ParseMissingField, field: "MAC"},
		{name: "invalid mac", data: []byte(`<ProbeMatch><Types>inquiry</Types><MAC>bad</MAC><IPv4Address>192.0.2.20</IPv4Address></ProbeMatch>`), limit: defaultLimit, kind: parser.ParseInvalidField, field: "MAC"},
		{name: "invalid ipv4", data: []byte(`<ProbeMatch><Types>inquiry</Types><MAC>00:11:22:33:44:55</MAC><IPv4Address>not-ip</IPv4Address></ProbeMatch>`), limit: defaultLimit, kind: parser.ParseInvalidField, field: "IPv4Address"},
		{name: "invalid activation", data: []byte(`<ProbeMatch><Types>inquiry</Types><MAC>00:11:22:33:44:55</MAC><IPv4Address>192.0.2.20</IPv4Address><Activated>flase</Activated></ProbeMatch>`), limit: defaultLimit, kind: parser.ParseInvalidField, field: "Activated"},
		{name: "conflicting aliases", data: []byte(`<ProbeMatch><Types>inquiry</Types><MAC>00:11:22:33:44:55</MAC><IPv4Address>192.0.2.20</IPv4Address><DeviceModel>A</DeviceModel><DeviceDescription>B</DeviceDescription></ProbeMatch>`), limit: defaultLimit, kind: parser.ParseInvalidField, field: "Model"},
		{name: "nil sender", data: referenceProbeMatch, limit: defaultLimit, kind: parser.ParseInvalidField, field: "sender"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotSender := net.Addr(sender)
			if testCase.name == "nil sender" {
				gotSender = nil
			}
			device, err := parser.ParseAdvertisement(testCase.data, gotSender, testCase.limit)
			assertParseFailure(t, device, err, testCase.kind, testCase.field)
		})
	}
}

func TestParseAdvertisementAcceptsExactSizeBoundaryAndIgnoresUnknownFields(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
	payload := append([]byte{}, referenceProbeMatch...)
	payload = append(payload[:len(payload)-len("</ProbeMatch>")], []byte("<Salt>opaque</Salt></ProbeMatch>")...)
	device, err := parser.ParseAdvertisement(payload, sender, len(payload))
	if err != nil {
		t.Fatalf("exact-size payload returned error: %v", err)
	}
	if device.SoftwareVersion != "V1.2.3" {
		t.Fatalf("SoftwareVersion = %q, want preserved version", device.SoftwareVersion)
	}
}

func TestParseAdvertisementErrorsAreSafeAndWrapCauses(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
	device, err := parser.ParseAdvertisement([]byte("<ProbeMatch>broken"), sender, defaultLimit)
	if err == nil {
		t.Fatal("expected malformed XML error")
	}
	if !reflect.DeepEqual(device, parser.Device{}) {
		t.Fatalf("device = %#v, want no usable device", device)
	}
	if strings.Contains(err.Error(), "ProbeMatch") {
		t.Fatalf("error disclosed packet content: %v", err)
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) || parseErr.Err == nil || !errors.Is(err, parseErr.Err) {
		t.Fatalf("error = %T %v, want ParseError wrapping a cause", err, err)
	}
}

func FuzzParseAdvertisementNeverPanics(f *testing.F) {
	f.Add(referenceProbeMatch)
	f.Add([]byte("<ProbeMatch>"))
	f.Add([]byte{0, 1, 2, 3, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		sender := &net.UDPAddr{IP: net.ParseIP(senderIP), Port: 37020}
		_, _ = parser.ParseAdvertisement(data, sender, defaultLimit)
	})
}

func assertDevice(t *testing.T, got parser.Device, mac, ipv4 string, activated bool, model, serial, version string, sender net.Addr) {
	t.Helper()
	if got.MAC.String() != mac || got.IPv4 != netip.MustParseAddr(ipv4) || got.Activated != activated ||
		got.Model != model || got.SerialNumber != serial || got.SoftwareVersion != version || got.Sender != sender {
		t.Fatalf("device = %#v, want MAC %s IPv4 %s active %t model %q serial %q version %q sender %v", got, mac, ipv4, activated, model, serial, version, sender)
	}
}

func assertParseFailure(t *testing.T, device parser.Device, err error, kind parser.ParseErrorKind, field string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual(device, parser.Device{}) {
		t.Fatalf("device = %#v, want no usable device", device)
	}
	var parseErr *parser.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error = %T %v, want ParseError", err, err)
	}
	if parseErr.Kind != kind || parseErr.Field != field {
		t.Fatalf("ParseError = %#v, want kind %q field %q", parseErr, kind, field)
	}
}
