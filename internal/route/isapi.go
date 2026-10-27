package route

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const defaultISAPIResponseLimit = 64 * 1024

type ISAPIHTTPTransport struct {
	Client           *http.Client
	BaseURL          string
	MaxResponseBytes int64
	Password         string
}

// ISAPIRouteProvider and ISAPICapabilityProvider are deliberately thin
// adapters: HTTP ownership and response bounds stay in this transport.
type ISAPIRouteProvider struct{ Transport *ISAPIHTTPTransport }

func (p ISAPIRouteProvider) CanReach(ctx context.Context, address netip.Addr, _ string) (bool, error) {
	if !address.IsValid() {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("target address is required")}
	}
	if p.Transport == nil {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI transport is required")}
	}
	_, err := p.Transport.probe(ctx, Target{Address: address})
	return err == nil, err
}

type CapabilityProfile struct {
	Operations map[Operation]bool
}

type ISAPICapabilityProvider struct{ Transport *ISAPIHTTPTransport }

func (p ISAPICapabilityProvider) Supports(ctx context.Context, target Target, operation Operation) (bool, error) {
	if p.Transport == nil {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI transport is required")}
	}
	profile, err := p.Transport.LookupCapabilities(ctx, target)
	if err != nil {
		return false, err
	}
	return profile.Operations[operation], nil
}

func (t *ISAPIHTTPTransport) probe(ctx context.Context, target Target) ([]byte, error) {
	base, err := t.baseURL()
	if err != nil {
		return nil, err
	}
	return t.request(ctx, http.MethodGet, base.ResolveReference(&url.URL{Path: "/ISAPI/System/capabilities"}), nil)
}

// LookupCapabilities performs one bounded capability/profile request. The
// parser intentionally accepts only explicit boolean operation fields.
func (t *ISAPIHTTPTransport) LookupCapabilities(ctx context.Context, target Target) (CapabilityProfile, error) {
	payload, err := t.probe(ctx, target)
	if err != nil {
		return CapabilityProfile{}, err
	}
	profile := CapabilityProfile{Operations: make(map[Operation]bool)}
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	for {
		token, decodeErr := decoder.Token()
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		var operation Operation
		switch strings.ToLower(start.Name.Local) {
		case "activation":
			operation = OperationActivation
		case "network":
			operation = OperationNetwork
		case "status":
			operation = OperationStatus
		case "discovery":
			operation = OperationDiscovery
		default:
			continue
		}
		var value string
		if decodeErr := decoder.DecodeElement(&value, &start); decodeErr != nil {
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
		}
		var supported bool
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1":
			supported = true
		case "false", "0":
			supported = false
		default:
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
		}
		if previous, exists := profile.Operations[operation]; exists && previous != supported {
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is contradictory")}
		}
		profile.Operations[operation] = supported
	}
	return profile, nil
}

func (t *ISAPIHTTPTransport) baseURL() (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(t.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" ||
		(base.Scheme != "http" && base.Scheme != "https") {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("valid ISAPI base URL is required")}
	}
	return base, nil
}

type securityChallengeRequest struct {
	XMLName   xml.Name `xml:"SecurityChallenge"`
	PublicKey string   `xml:"PublicKey>key"`
}

type securityChallengeResponse struct {
	Challenge string `xml:"key"`
}

type activateInfo struct {
	XMLName  xml.Name `xml:"ActivateInfo"`
	Password string   `xml:"password"`
}

type activationStatus struct {
	Activated string `xml:"Activated"`
}

func (t *ISAPIHTTPTransport) Execute(ctx context.Context, operation Operation, target Target) (Result, error) {
	if !target.Address.IsValid() {
		return Result{Operation: operation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorInvalidInput, Err: errors.New("target address is required")}
	}
	base, err := t.baseURL()
	if err != nil {
		return Result{Operation: operation, Target: target, Protocol: "isapi"}, err
	}
	if operation == OperationActivation {
		return t.activate(ctx, base, target)
	}
	if operation == OperationStatus {
		return t.status(ctx, base, target)
	}
	return Result{Operation: operation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI operation is not implemented")}
}

func (t *ISAPIHTTPTransport) activate(ctx context.Context, base *url.URL, target Target) (Result, error) {
	material, err := NewActivationSecurityMaterial(t.Password)
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	requestBody, err := xml.Marshal(securityChallengeRequest{PublicKey: material.PublicKey})
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorMalformed, Err: err}
	}
	challengeURL := base.ResolveReference(&url.URL{Path: "/ISAPI/Security/challenge"})
	challengePayload, err := t.request(ctx, http.MethodPost, challengeURL, requestBody)
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	var challenge securityChallengeResponse
	if err := xml.Unmarshal(challengePayload, &challenge); err != nil || challenge.Challenge == "" {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI challenge response is malformed")}
	}
	password, err := BuildISAPIActivationPassword(material, challenge.Challenge)
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	activationPayload, err := xml.Marshal(activateInfo{Password: password})
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorMalformed, Err: err}
	}
	activateURL := base.ResolveReference(&url.URL{Path: "/ISAPI/System/activate"})
	if _, err := t.request(ctx, http.MethodPut, activateURL, activationPayload); err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	status, err := t.status(ctx, base, target)
	if err != nil {
		status.Operation = OperationActivation
		return status, err
	}
	status.Operation = OperationActivation
	return status, nil
}

func (t *ISAPIHTTPTransport) status(ctx context.Context, base *url.URL, target Target) (Result, error) {
	statusURL := base.ResolveReference(&url.URL{Path: "/SDK/activateStatus"})
	payload, err := t.request(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return Result{Operation: OperationStatus, Target: target, Protocol: "isapi"}, err
	}
	var status activationStatus
	if err := xml.Unmarshal(payload, &status); err != nil {
		return Result{Operation: OperationStatus, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI activation status is malformed")}
	}
	if !strings.EqualFold(strings.TrimSpace(status.Activated), "true") && status.Activated != "1" {
		return Result{Operation: OperationStatus, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorVerification, Err: errors.New("activation verification failed")}
	}
	return Result{Operation: OperationStatus, Target: target, Protocol: "isapi"}, nil
}

func (t *ISAPIHTTPTransport) request(ctx context.Context, method string, endpoint *url.URL, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Err: err}
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/xml")
	}
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		kind := ErrorTimeout
		if ctx.Err() != nil {
			kind = ErrorCanceled
			err = ctx.Err()
		}
		return nil, &Error{Kind: kind, Err: err}
	}
	defer response.Body.Close()
	limit := t.MaxResponseBytes
	if limit <= 0 {
		limit = defaultISAPIResponseLimit
	}
	payload, readErr := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if readErr != nil {
		return nil, &Error{Kind: ErrorMalformed, Err: readErr}
	}
	if int64(len(payload)) > limit {
		return nil, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI response exceeds configured limit")}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, &Error{Kind: ErrorAuthentication, Err: fmt.Errorf("ISAPI HTTP status %s", strconv.Itoa(response.StatusCode))}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{Kind: ErrorRejected, Err: fmt.Errorf("ISAPI HTTP status %s", strconv.Itoa(response.StatusCode))}
	}
	return payload, nil
}
