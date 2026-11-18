package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestISAPIDiscoveryTextOutputDoesNotDependOnSADPResults(t *testing.T) {
	for _, tt := range []struct {
		name      string
		discovery []DiscoveredDevice
	}{
		{name: "without SADP results"},
		{name: "with SADP results", discovery: []DiscoveredDevice{{MAC: "01", IPv4: "192.0.2.1"}, {MAC: "02", IPv4: "192.0.2.2"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := CommandResult{
				OK: true, Discovery: tt.discovery,
				ISAPIDevice: &ISAPIDiscoveredDevice{
					MAC: "00:01:02:03:04:05", Address: "192.0.2.10", Model: "Synthetic",
				},
			}

			var output bytes.Buffer
			if err := writeResult(&output, FormatText, result); err != nil {
				t.Fatal(err)
			}
			if count := strings.Count(output.String(), "00:01:02:03:04:05 192.0.2.10 Synthetic"); count != 1 {
				t.Fatalf("ISAPI device appears %d times in %q", count, output.String())
			}
		})
	}
}

func TestSADPDiscoveryReportsSerialAndKnownActivation(t *testing.T) {
	for _, tt := range []struct {
		known  bool
		active bool
		want   string
	}{
		{false, false, "unknown"},
		{true, false, "inactive"},
		{true, true, "active"},
	} {
		result := CommandResult{Discovery: []DiscoveredDevice{{
			MAC: "00:01:02:03:04:05", IPv4: "192.0.2.1", SerialNumber: "synthetic-serial",
			ActivationKnown: tt.known, Activated: tt.active,
		}}}
		var output bytes.Buffer
		if err := writeResult(&output, FormatText, result); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(output.String(), "serial_number=synthetic-serial activation="+tt.want) {
			t.Fatalf("unexpected output %q", output.String())
		}
		converted, ok := toParserDevice(DiscoveredDevice{
			MAC: "00:01:02:03:04:05", IPv4: "192.0.2.1", Sender: "192.0.2.1",
			SerialNumber: "synthetic-serial", ActivationKnown: tt.known, Activated: tt.active,
		})
		if !ok || converted.SerialNumber != "synthetic-serial" || converted.ActivationKnown != tt.known {
			t.Fatalf("unexpected device conversion: %+v", converted)
		}
	}
}
