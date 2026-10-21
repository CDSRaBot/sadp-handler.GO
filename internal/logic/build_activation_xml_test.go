package logic_test

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestBuildActivationXMLSerializesRequiredFields(t *testing.T) {
	request := logic.ActivationRequest{
		MAC:      net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Password: "ValidPass1!",
	}

	payload, err := logic.BuildActivationXML(request)
	if err != nil {
		t.Fatalf("BuildActivationXML returned error: %v", err)
	}
	if !bytes.HasPrefix(payload, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("payload does not start with xml.Header: %q", payload)
	}

	document := parseActivationXML(t, payload)
	if document.XMLName.Local != "Activate" {
		t.Fatalf("root = %q, want Activate", document.XMLName.Local)
	}
	if document.TargetMAC != "00:11:22:33:44:55" {
		t.Fatalf("TargetMAC = %q", document.TargetMAC)
	}
	if document.Password != request.Password {
		t.Fatal("Password XML element does not preserve the supplied value")
	}
	if document.OptionData != nil {
		t.Fatal("OptionData must be omitted when Email is empty")
	}
}

func TestBuildActivationXMLIncludesAndEscapesOptionalSecurityEmail(t *testing.T) {
	request := logic.ActivationRequest{
		MAC:      net.HardwareAddr{10, 11, 12, 13, 14, 15},
		Password: `Valid&Pass1!`,
		Email:    `ops&security@example.test`,
	}

	payload, err := logic.BuildActivationXML(request)
	if err != nil {
		t.Fatalf("BuildActivationXML returned error: %v", err)
	}
	if !bytes.Contains(payload, []byte(`Valid&amp;Pass1!`)) {
		t.Fatalf("password was not XML escaped: %q", payload)
	}
	if !bytes.Contains(payload, []byte(`ops&amp;security@example.test`)) {
		t.Fatalf("email was not XML escaped: %q", payload)
	}

	document := parseActivationXML(t, payload)
	if document.OptionData == nil || document.OptionData.SecurityEmail != request.Email {
		t.Fatalf("SecurityEmail = %#v, want %q", document.OptionData, request.Email)
	}
}

func TestBuildActivationXMLRejectsInvalidRequestsWithoutPayload(t *testing.T) {
	validMAC := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	testCases := []struct {
		name    string
		request logic.ActivationRequest
	}{
		{
			name: "empty MAC",
			request: logic.ActivationRequest{
				Password: "ValidPass1!",
			},
		},
		{
			name: "non EUI-48 MAC",
			request: logic.ActivationRequest{
				MAC:      net.HardwareAddr{0, 17, 34, 51, 68, 85, 102, 119},
				Password: "ValidPass1!",
			},
		},
		{
			name: "invalid password",
			request: logic.ActivationRequest{
				MAC:      validMAC,
				Password: "short",
			},
		},
		{
			name: "empty password",
			request: logic.ActivationRequest{
				MAC: validMAC,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := logic.BuildActivationXML(testCase.request)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if len(payload) != 0 {
				t.Fatalf("payload = %q, want no usable payload", payload)
			}
			var operationErr *logic.DeviceOperationError
			if !errors.As(err, &operationErr) {
				t.Fatalf("error = %T %v, want DeviceOperationError", err, err)
			}
			if operationErr.Kind != logic.OperationInvalidInput {
				t.Fatalf("error kind = %q, want %q", operationErr.Kind, logic.OperationInvalidInput)
			}
		})
	}
}

func TestBuildActivationXMLDoesNotDiscloseSensitiveValuesInErrors(t *testing.T) {
	const password = "activation-password-canary-123!"
	const email = "activation-email-canary@example.test"

	_, err := logic.BuildActivationXML(logic.ActivationRequest{
		MAC:      net.HardwareAddr{1, 2, 3},
		Password: password,
		Email:    email,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), email) {
		t.Fatalf("error disclosed a secret: %v", err)
	}
	var operationErr *logic.DeviceOperationError
	if !errors.As(err, &operationErr) || operationErr.Kind != logic.OperationInvalidInput {
		t.Fatalf("error = %T %v, want invalid-input DeviceOperationError", err, err)
	}
}
