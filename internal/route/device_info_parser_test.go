package route

import "testing"

func TestISAPIDeviceInfoRejectsAmbiguousIdentity(t *testing.T) {
	tests := []string{
		`<DeviceInfo><macAddress>00:01:02:03:04:05</macAddress><macAddress>00:01:02:03:04:06</macAddress></DeviceInfo>`,
		`<DeviceInfo><macAddress><value>00:01:02:03:04:05</value></macAddress></DeviceInfo>`,
		`<DeviceInfo/><DeviceInfo/>`,
		`<Other><macAddress>00:01:02:03:04:05</macAddress></Other>`,
	}
	for _, payload := range tests {
		if _, err := parseISAPIDeviceInfo([]byte(payload)); err == nil {
			t.Fatalf("ambiguous device info accepted: %s", payload)
		}
	}
}

func TestISAPIDeviceInfoAcceptsNamespacedDocument(t *testing.T) {
	payload := `<DeviceInfo xmlns="http://www.isapi.org/ver20/XMLSchema"><macAddress>00:01:02:03:04:05</macAddress><other><model>ignored</model></other><model>TEST</model></DeviceInfo>`
	info, err := parseISAPIDeviceInfo([]byte(payload))
	if err != nil || info.MACAddress != "00:01:02:03:04:05" || info.Model != "TEST" {
		t.Fatalf("info = %#v, error = %v", info, err)
	}
}
