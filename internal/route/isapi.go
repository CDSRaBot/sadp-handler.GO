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
	AllowBasic       bool
	Verifier         ISAPIActivationVerifier
}

type ISAPIActivationVerifier interface {
	VerifyActivation(context.Context, Target, string) error
}

type ISAPIDeviceInfoVerifier struct {
	Transport *ISAPIHTTPTransport
}

type ISAPIActivationState uint8

const (
	ISAPIActivationInactive ISAPIActivationState = iota + 1
	ISAPIActivationActive
)

func (v ISAPIDeviceInfoVerifier) VerifyActivation(ctx context.Context, target Target, password string) error {
	if v.Transport == nil || v.Transport.Username == "" || len(target.MAC) != 6 || password == "" {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("device identity and credentials are required")}
	}
	transport := *v.Transport
	transport.Password = password
	_, err := transport.discover(ctx, target)
	return err
}

type isapiDeviceInfo struct {
	MACAddress      string
	Model           string
	SerialNumber    string
	FirmwareVersion string
}

func parseISAPIDeviceInfo(payload []byte) (isapiDeviceInfo, error) {
	var info isapiDeviceInfo
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	seen := make(map[string]bool)
	depth := 0
	rootSeen := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if rootSeen && depth == 0 {
				return info, nil
			}
			return isapiDeviceInfo{}, errors.New("invalid device information")
		}
		if err != nil {
			return isapiDeviceInfo{}, err
		}
		switch token := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return isapiDeviceInfo{}, errors.New("invalid device information")
			}
		case xml.EndElement:
			depth--
		case xml.StartElement:
			depth++
			if depth == 1 {
				if rootSeen || token.Name.Local != "DeviceInfo" {
					return isapiDeviceInfo{}, errors.New("invalid device information root")
				}
				rootSeen = true
				continue
			}
			if depth != 2 {
				continue
			}
			var destination *string
			switch token.Name.Local {
			case "macAddress":
				destination = &info.MACAddress
			case "model":
				destination = &info.Model
			case "serialNumber":
				destination = &info.SerialNumber
			case "firmwareVersion":
				destination = &info.FirmwareVersion
			default:
				continue
			}
			if seen[token.Name.Local] {
				return isapiDeviceInfo{}, errors.New("duplicate device information field")
			}
			seen[token.Name.Local] = true
			value, err := decodeSimpleElement(decoder)
			if err != nil {
				return isapiDeviceInfo{}, err
			}
			*destination = value
			depth--
		}
	}
}

func (t *ISAPIHTTPTransport) discover(ctx context.Context, target Target) (Result, error) {
	result := Result{Operation: OperationDiscovery, Protocol: "isapi", Target: target}
	if t.Username == "" || t.Password == "" {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("device credentials are required")}
	}
	if len(target.MAC) != 0 && len(target.MAC) != 6 {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("target MAC must be EUI-48")}
	}
	base, err := t.baseURL(target)
	if err != nil {
		return result, err
	}
	endpoint := base.ResolveReference(&url.URL{Path: "/ISAPI/System/deviceInfo"})
	payload, err := t.requestWithPolicy(ctx, http.MethodGet, endpoint, nil, true)
	if err != nil {
		return result, err
	}
	info, err := parseISAPIDeviceInfo(payload)
	if err != nil {
		return result, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI device information is malformed")}
	}
	mac, err := net.ParseMAC(strings.TrimSpace(info.MACAddress))
	if err != nil || len(mac) != 6 || len(target.MAC) != 0 && !bytes.Equal(mac, target.MAC) {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI device identity did not match")}
	}
	result.Target.MAC = append(net.HardwareAddr(nil), mac...)
	result.Device = &DeviceInfo{
		Model:           strings.TrimSpace(info.Model),
		SerialNumber:    strings.TrimSpace(info.SerialNumber),
		FirmwareVersion: strings.TrimSpace(info.FirmwareVersion),
	}
	return result, nil
}

// ISAPIRouteProvider and ISAPICapabilityProvider are deliberately thin
// adapters: HTTP ownership and response bounds stay in this transport.
type ISAPIRouteProvider struct{ Transport *ISAPIHTTPTransport }

func (p ISAPIRouteProvider) CanReach(ctx context.Context, address netip.Addr, iface string) (bool, error) {
	if ctx == nil {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("context is required")}
	}
	if !address.IsValid() {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("target address is required")}
	}
	if p.Transport == nil {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI transport is required")}
	}
	if iface != "" {
		return false, &Error{Kind: ErrorUnsupported, Err: errors.New("explicit ISAPI interface binding is not implemented")}
	}
	base, err := p.Transport.baseURL(Target{Address: address})
	if err != nil {
		return false, err
	}
	port := base.Port()
	if port == "" {
		port = "80"
		if base.Scheme == "https" {
			port = "443"
		}
	}
	dialer := net.Dialer{Timeout: defaultISAPIRequestTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), port))
	if err != nil {
		if ctx.Err() != nil {
			return false, contextError(ctx.Err())
		}
		var networkErr net.Error
		if errors.As(err, &networkErr) && networkErr.Timeout() {
			return false, &Error{Kind: ErrorTimeout, Err: errors.New("ISAPI route probe timed out")}
		}
		return false, &Error{Kind: ErrorNoRoute, Err: errors.New("ISAPI target is not reachable")}
	}
	if err := conn.Close(); err != nil {
		return false, &Error{Kind: ErrorTransport, Err: errors.New("failed to close route probe")}
	}
	return true, nil
}

type CapabilityProfile struct {
	Operations map[Operation]bool
}

type ISAPICapabilityProvider struct{ Transport *ISAPIHTTPTransport }

func (p ISAPICapabilityProvider) Supports(ctx context.Context, target Target, operation Operation) (bool, error) {
	if p.Transport == nil {
		return false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI transport is required")}
	}
	if operation == OperationDiscovery {
		_, err := p.Transport.discover(ctx, target)
		return err == nil, err
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
	depth := 0
	rootSeen := false
	for {
		token, decodeErr := decoder.Token()
		if decodeErr == io.EOF {
			if !rootSeen || depth != 0 {
				return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
			}
			break
		}
		if decodeErr != nil {
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
		}
		if _, ok := token.(xml.EndElement); ok {
			depth--
			continue
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		depth++
		if depth == 1 {
			if rootSeen || start.Name.Local != "Capabilities" {
				return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response has an unsupported root")}
			}
			rootSeen = true
			continue
		}
		if depth != 2 {
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
		value, decodeErr := decodeSimpleElement(decoder)
		if decodeErr != nil {
			return CapabilityProfile{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI capability response is malformed")}
		}
		depth--
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

func decodeSimpleElement(decoder *xml.Decoder) (string, error) {
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch token := token.(type) {
		case xml.CharData:
			value.Write(token)
		case xml.EndElement:
			return value.String(), nil
		case xml.StartElement:
			return "", errors.New("nested text field")
		}
	}
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
	XMLName   xml.Name `xml:"PublicKey"`
	PublicKey string   `xml:"key"`
}

type activateInfo struct {
	XMLName  xml.Name `xml:"ActivateInfo"`
	Password string   `xml:"password"`
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
	if operation == OperationDiscovery {
		return t.discover(ctx, target)
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
	challenge, err := parseUniqueDirectText(challengePayload, "Challenge", "key")
	if err != nil || strings.TrimSpace(challenge) == "" {
		return Result{Operation: OperationActivation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI challenge response is malformed")}
	}
	password, err := BuildISAPIActivationPassword(material, challenge)
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
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	var status string
	depth := 0
	rootSeen := false
	statusSeen := false
	malformed := func() error {
		return &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI response status is malformed")}
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return malformed()
		}
		switch token := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return malformed()
			}
		case xml.EndElement:
			depth--
		case xml.StartElement:
			depth++
			if depth == 1 {
				if rootSeen || token.Name.Local != "ResponseStatus" {
					return malformed()
				}
				rootSeen = true
				continue
			}
			if token.Name.Local != "statusCode" {
				continue
			}
			if depth != 2 || statusSeen {
				return malformed()
			}
			statusSeen = true
			status, err = decodeSimpleElement(decoder)
			if err != nil {
				return malformed()
			}
			depth--
		}
	}
	if !rootSeen || depth != 0 || !statusSeen || strings.TrimSpace(status) == "" {
		return malformed()
	}
	if strings.TrimSpace(status) != "1" {
		return &Error{Kind: ErrorRejected, Err: errors.New("ISAPI operation was rejected")}
	}
	return nil
}

func (t *ISAPIHTTPTransport) status(ctx context.Context, base *url.URL, target Target) (Result, error) {
	state, err := t.readActivationState(ctx, base)
	result := Result{Operation: OperationStatus, Target: target, Protocol: "isapi"}
	if err != nil {
		return result, err
	}
	if state != ISAPIActivationActive {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("activation verification failed")}
	}
	return result, nil
}

// ReadActivationState is a read-only preflight signal, not proof of device
// identity or a substitute for authenticated verification after activation.
func (t *ISAPIHTTPTransport) ReadActivationState(ctx context.Context, target Target) (ISAPIActivationState, error) {
	if ctx == nil || t == nil {
		return 0, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI activation status dependencies are incomplete")}
	}
	if target.Interface != "" {
		return 0, &Error{Kind: ErrorUnsupported, Err: errors.New("explicit ISAPI interface binding is not implemented")}
	}
	base, err := t.baseURL(target)
	if err != nil {
		return 0, err
	}
	return t.readActivationState(ctx, base)
}

func (t *ISAPIHTTPTransport) readActivationState(ctx context.Context, base *url.URL) (ISAPIActivationState, error) {
	statusURL := base.ResolveReference(&url.URL{Path: "/SDK/activateStatus"})
	payload, err := t.request(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return 0, err
	}
	status, err := parseUniqueDirectText(payload, "", "Activated")
	if err != nil {
		return 0, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI activation status is malformed")}
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "true", "1":
		return ISAPIActivationActive, nil
	case "false", "0":
		return ISAPIActivationInactive, nil
	default:
		return 0, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI activation status is malformed")}
	}
}

func parseUniqueDirectText(payload []byte, root, field string) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	var value string
	depth := 0
	rootSeen, fieldSeen := false, false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if rootSeen && depth == 0 && fieldSeen {
				return value, nil
			}
			return "", errors.New("missing XML root or field")
		}
		if err != nil {
			return "", err
		}
		switch token := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return "", errors.New("text outside XML root")
			}
		case xml.EndElement:
			depth--
		case xml.StartElement:
			depth++
			if depth == 1 {
				if rootSeen || (root != "" && token.Name.Local != root) {
					return "", errors.New("invalid XML root")
				}
				rootSeen = true
				continue
			}
			if token.Name.Local != field {
				continue
			}
			if depth != 2 || fieldSeen {
				return "", errors.New("ambiguous XML field")
			}
			fieldSeen = true
			value, err = decodeSimpleElement(decoder)
			if err != nil {
				return "", err
			}
			depth--
		}
	}
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
	client := t.boundedHTTPClient()
	status, headers, payload, err := t.performRequest(ctx, client, request)
	if err != nil {
		return nil, err
	}
	if requireAuthentication && status != http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorAuthentication, Err: errors.New("ISAPI authentication challenge is required")}
	}
	if status == http.StatusUnauthorized && t.Username != "" {
		authorization, authErr := t.authorizationForChallenge(headers.Values("WWW-Authenticate"), request)
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
		status, _, payload, err = t.performRequest(ctx, client, retry)
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

func (t *ISAPIHTTPTransport) boundedHTTPClient() *http.Client {
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	if copy.Timeout <= 0 {
		copy.Timeout = defaultISAPIRequestTimeout
	}
	copy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &copy
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
