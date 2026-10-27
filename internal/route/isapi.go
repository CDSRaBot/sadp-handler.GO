package route

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	base, err := url.Parse(strings.TrimRight(t.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return Result{Operation: operation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorInvalidInput, Err: errors.New("valid ISAPI base URL is required")}
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return Result{Operation: operation, Target: target, Protocol: "isapi"}, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI URL must use HTTP or HTTPS")}
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
