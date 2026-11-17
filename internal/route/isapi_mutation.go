package route

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
)

const maxISAPIMutationBody = 64 * 1024

func (t *ISAPIHTTPTransport) authenticatedMutation(ctx context.Context, method string, endpoint *url.URL, body []byte) ([]byte, bool, error) {
	if ctx == nil || endpoint == nil || t.Username == "" || t.Password == "" ||
		(method != http.MethodPut && method != http.MethodPost) || len(body) > maxISAPIMutationBody {
		return nil, false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI mutation arguments are invalid")}
	}
	client := t.boundedHTTPClient()
	probe, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI mutation endpoint is invalid")}
	}
	status, headers, _, err := t.performRequest(ctx, client, probe)
	if err != nil {
		return nil, false, err
	}
	if status != http.StatusUnauthorized {
		return nil, false, &Error{Kind: ErrorAuthentication, Err: errors.New("ISAPI mutation requires an authentication challenge")}
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, false, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI mutation request is invalid")}
	}
	authorization, err := t.authorizationForChallenge(headers.Values("WWW-Authenticate"), request)
	if err != nil {
		return nil, false, &Error{Kind: ErrorAuthentication, Err: err}
	}
	request.Header.Set("Authorization", authorization)
	if body != nil {
		request.Header.Set("Content-Type", "application/xml")
	}
	status, _, payload, err := t.performRequest(ctx, client, request)
	if err != nil {
		return nil, true, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, true, &Error{Kind: ErrorAuthentication, Err: errors.New("ISAPI mutation authentication failed")}
	}
	if status < 200 || status >= 300 {
		return nil, true, &Error{Kind: ErrorRejected, Err: errors.New("ISAPI mutation was rejected")}
	}
	return payload, true, nil
}
