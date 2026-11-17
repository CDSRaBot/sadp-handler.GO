package route

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultISAPIResponseLimit = 64 * 1024
const defaultISAPIRequestTimeout = 10 * time.Second

type ISAPIHTTPTransport struct {
	Client           *http.Client
	BaseURL          string
	MaxResponseBytes int64
	Username         string
	Password         string
	Verifier         ISAPIActivationVerifier
}

type ISAPIActivationVerifier interface {
	VerifyActivation(context.Context, Target, string) error
}

type ISAPIDeviceInfoVerifier struct {
	Transport *ISAPIHTTPTransport
}

func (v ISAPIDeviceInfoVerifier) VerifyActivation(ctx context.Context, target Target, password string) error {
	if v.Transport == nil || v.Transport.Username == "" || len(target.MAC) != 6 || password == "" {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("device identity and credentials are required")}
	}
	transport := *v.Transport
	transport.Password = password
	base, err := transport.baseURL(target)
	if err != nil {
		return err
	}
	endpoint := base.ResolveReference(&url.URL{Path: "/ISAPI/System/deviceInfo"})
	payload, err := transport.requestWithPolicy(ctx, http.MethodGet, endpoint, nil, true)
	if err != nil {
		return err
	}
	var info struct {
		XMLName    xml.Name `xml:"DeviceInfo"`
		MACAddress string   `xml:"macAddress"`
	}
	if err := xml.Unmarshal(payload, &info); err != nil {
		return &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI device information is malformed")}
	}
	mac, err := net.ParseMAC(strings.TrimSpace(info.MACAddress))
	if err != nil || len(mac) != 6 || !bytes.Equal(mac, target.MAC) {
		return &Error{Kind: ErrorVerification, Err: errors.New("ISAPI device identity did not match")}
	}
	return nil
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
	base, err := t.baseURL(target)
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

func (t *ISAPIHTTPTransport) baseURL(target Target) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(t.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" ||
		(base.Scheme != "http" && base.Scheme != "https") ||
		base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("valid ISAPI base URL is required")}
	}
	if !target.Address.IsValid() || target.Address.IsUnspecified() || target.Address.IsMulticast() {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("valid unicast target address is required")}
	}
	if port := base.Port(); port != "" {
		base.Host = net.JoinHostPort(target.Address.String(), port)
	} else if target.Address.Is6() {
		base.Host = "[" + target.Address.String() + "]"
	} else {
		base.Host = target.Address.String()
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

type responseStatus struct {
	XMLName    xml.Name `xml:"ResponseStatus"`
	StatusCode string   `xml:"statusCode"`
}

func (t *ISAPIHTTPTransport) Execute(ctx context.Context, operation Operation, target Target) (Result, error) {
	if !target.Address.IsValid() {
		return Result{Operation: operation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorInvalidInput, Err: errors.New("target address is required")}
	}
	base, err := t.baseURL(target)
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
	if t.Verifier == nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI activation identity verifier is required")}
	}
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
	response, err := t.request(ctx, http.MethodPut, activateURL, activationPayload)
	if err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	if err := validateISAPIResponseStatus(response); err != nil {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, err
	}
	status, err := t.status(ctx, base, target)
	if err != nil {
		status.Operation = OperationActivation
		return status, err
	}
	status.Operation = OperationActivation
	if err := t.Verifier.VerifyActivation(ctx, target, t.Password); err != nil {
		return status, &Error{Kind: ErrorVerification, Err: err}
	}
	return status, nil
}

func validateISAPIResponseStatus(payload []byte) error {
	var response responseStatus
	if err := xml.Unmarshal(payload, &response); err != nil || strings.TrimSpace(response.StatusCode) == "" {
		return &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI response status is malformed")}
	}
	if strings.TrimSpace(response.StatusCode) != "1" {
		return &Error{Kind: ErrorRejected, Err: errors.New("ISAPI operation was rejected")}
	}
	return nil
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
	return t.requestWithPolicy(ctx, method, endpoint, body, false)
}

func (t *ISAPIHTTPTransport) requestWithPolicy(ctx context.Context, method string, endpoint *url.URL, body []byte, requireAuthentication bool) ([]byte, error) {
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
	boundedClient := *client
	if boundedClient.Timeout <= 0 {
		boundedClient.Timeout = defaultISAPIRequestTimeout
	}
	boundedClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	status, headers, payload, err := t.performRequest(ctx, &boundedClient, request)
	if err != nil {
		return nil, err
	}
	if requireAuthentication && status != http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorAuthentication, Err: errors.New("ISAPI Digest authentication challenge is required")}
	}
	if status == http.StatusUnauthorized && t.Username != "" {
		authorization, authErr := digestAuthorization(headers.Values("WWW-Authenticate"), request, t.Username, t.Password)
		if authErr != nil {
			return nil, &Error{Kind: ErrorAuthentication, Err: authErr}
		}
		retry := request.Clone(ctx)
		if request.GetBody != nil {
			retry.Body, err = request.GetBody()
			if err != nil {
				return nil, &Error{Kind: ErrorTransport, Err: errors.New("ISAPI request body cannot be replayed")}
			}
		}
		retry.Header.Set("Authorization", authorization)
		status, _, payload, err = t.performRequest(ctx, &boundedClient, retry)
		if err != nil {
			return nil, err
		}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, &Error{Kind: ErrorAuthentication, Err: fmt.Errorf("ISAPI HTTP status %s", strconv.Itoa(status))}
	}
	if status < 200 || status >= 300 {
		return nil, &Error{Kind: ErrorRejected, Err: fmt.Errorf("ISAPI HTTP status %s", strconv.Itoa(status))}
	}
	return payload, nil
}

func (t *ISAPIHTTPTransport) performRequest(ctx context.Context, client *http.Client, request *http.Request) (int, http.Header, []byte, error) {
	response, err := client.Do(request)
	if err != nil {
		kind := ErrorTransport
		if errors.Is(ctx.Err(), context.Canceled) {
			kind = ErrorCanceled
			err = ctx.Err()
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = ErrorTimeout
			err = ctx.Err()
		} else {
			var networkErr net.Error
			if errors.As(err, &networkErr) && networkErr.Timeout() {
				kind = ErrorTimeout
			}
		}
		return 0, nil, nil, &Error{Kind: kind, Err: err}
	}
	defer response.Body.Close()
	limit := t.MaxResponseBytes
	if limit <= 0 {
		limit = defaultISAPIResponseLimit
	}
	payload, readErr := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if readErr != nil {
		if ctx.Err() != nil {
			return 0, nil, nil, contextError(ctx.Err())
		}
		return 0, nil, nil, &Error{Kind: ErrorMalformed, Err: readErr}
	}
	if int64(len(payload)) > limit {
		return 0, nil, nil, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI response exceeds configured limit")}
	}
	return response.StatusCode, response.Header.Clone(), payload, nil
}
