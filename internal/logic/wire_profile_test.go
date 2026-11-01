package logic_test

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestBuildNetworkXMLWithSaltedProfileUsesInjectedEncoder(t *testing.T) {
	payload, err := logic.BuildNetworkXMLWithProfile(
		net.HardwareAddr{0, 17, 34, 51, 68, 85},
		logic.NetworkConfig{UseDHCP: true},
		"challenge",
		"password",
		logic.SADPWireOptions{
			Profile: logic.SADPWireProfileSaltedAES,
			Encode: func(value, salt string) (string, error) {
				if value != "password" || salt != "challenge" {
					t.Fatalf("encoder inputs = %q/%q", value, salt)
				}
				return "verified-ciphertext", nil
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildNetworkXMLWithProfile returned error: %v", err)
	}
	if !strings.Contains(string(payload), "verified-ciphertext") {
		t.Fatalf("payload does not contain injected encoded field: %q", payload)
	}
	if strings.Contains(string(payload), "password") {
		t.Fatal("payload contains plaintext password")
	}
}

func TestBuildNetworkXMLWithSaltedProfileFailsClosedWithoutCodec(t *testing.T) {
	payload, err := logic.BuildNetworkXMLWithProfile(
		net.HardwareAddr{0, 17, 34, 51, 68, 85},
		logic.NetworkConfig{UseDHCP: true},
		"challenge",
		"password",
		logic.SADPWireOptions{Profile: logic.SADPWireProfileSaltedAES},
	)
	if len(payload) != 0 || err == nil {
		t.Fatalf("payload/error = %q/%v, want empty payload and error", payload, err)
	}
	var operationErr *logic.DeviceOperationError
	if !errors.As(err, &operationErr) || operationErr.Kind != logic.OperationInvalidInput {
		t.Fatalf("error = %T %v, want invalid-input operation error", err, err)
	}
}

func TestBuildOptionalWorkflowsAllowQuestionsToBeOmitted(t *testing.T) {
	mac := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	if payload, err := logic.BuildHikConnectXML(logic.HikConnectRequest{MAC: mac, Enabled: true}); err != nil || len(payload) == 0 {
		t.Fatalf("BuildHikConnectXML payload/error = %q/%v", payload, err)
	}
	if payload, err := logic.BuildSecurityQuestionsXML(logic.SecurityQuestionsRequest{MAC: mac}); err == nil || len(payload) != 0 {
		t.Fatalf("empty questions payload/error = %q/%v", payload, err)
	}
}
