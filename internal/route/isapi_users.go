package route

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ISAPIUserOperation string

const (
	ISAPIUserCreate ISAPIUserOperation = "create"
	ISAPIUserUpdate ISAPIUserOperation = "update"
)

type ISAPIUserRequest struct {
	Operation ISAPIUserOperation
	ID        string
	Name      string
	Password  string
}

// ISAPIUserProfile is an independently verified device-specific capability,
// key source, XML schema and postcondition for encrypted user management.
// EncryptionKey transfers ownership of its returned key to the workflow.
type ISAPIUserProfile interface {
	VerifyUserDevice(context.Context, Target, DeviceInfo, ISAPIUserOperation) error
	EncryptionKey(context.Context, *ISAPIHTTPTransport, Target, ISAPIUserOperation) (ISAPIUserSecurity, []byte, error)
	BuildUserMutation(ISAPIUserRequest, ISAPIEncryptedField) (method string, body []byte, err error)
	VerifyUserState(ISAPIUserRequest, []byte) error
}

type ISAPIUserWorkflow struct {
	Transport *ISAPIHTTPTransport
	Profile   ISAPIUserProfile
}

type ISAPIUserResult struct {
	MutationAttempted bool
	Verified          bool
}

// ValidateISAPIUserRequest checks operation-independent user fields. Device
// profiles may enforce more restrictive rules before building their payload.
func ValidateISAPIUserRequest(request ISAPIUserRequest) error {
	if err := ValidateISAPIUserIdentity(request.Operation, request.ID, request.Name); err != nil {
		return err
	}
	if !validISAPIUserText(request.Password, MaxISAPIUserFieldBytes) {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user password is invalid")}
	}
	return nil
}

// ValidateISAPIUserIdentity checks user fields that do not contain a secret.
func ValidateISAPIUserIdentity(operation ISAPIUserOperation, id, name string) error {
	if operation != ISAPIUserCreate && operation != ISAPIUserUpdate {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user operation is invalid")}
	}
	if len(id) == 0 || len(id) > 10 || id[0] == '0' {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user ID is invalid")}
	}
	for _, character := range id {
		if character < '0' || character > '9' {
			return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user ID is invalid")}
		}
	}
	if _, err := strconv.ParseUint(id, 10, 32); err != nil {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user ID is invalid")}
	}
	if !validISAPIUserText(name, 128) || strings.TrimSpace(name) != name {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user name is invalid")}
	}
	return nil
}

func validISAPIUserText(value string, limit int) bool {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func (w ISAPIUserWorkflow) Configure(ctx context.Context, target Target, request ISAPIUserRequest) (ISAPIUserResult, error) {
	result := ISAPIUserResult{}
	if ctx == nil || w.Transport == nil || w.Transport.Username == "" ||
		w.Transport.Password == "" || len(target.MAC) != 6 || target.Interface != "" {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user workflow arguments are incomplete")}
	}
	if err := ValidateISAPIUserRequest(request); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, contextError(err)
	}
	if w.Profile == nil {
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("verified ISAPI user profile is required")}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	discovered, err := w.Transport.discover(ctx, target)
	if err != nil {
		return result, err
	}
	if discovered.Device == nil {
		return result, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI device information is missing")}
	}
	gateTarget := target
	gateTarget.MAC = append([]byte(nil), target.MAC...)
	if err := w.Profile.VerifyUserDevice(ctx, gateTarget, *discovered.Device, request.Operation); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, contextError(err)
		}
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI user profile is not verified for this device")}
	}
	if !bytes.Equal(gateTarget.MAC, target.MAC) {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI user profile changed the target identity")}
	}
	keyTarget := target
	keyTarget.MAC = append([]byte(nil), target.MAC...)
	keyTransport := *w.Transport
	scheme, key, err := w.Profile.EncryptionKey(ctx, &keyTransport, keyTarget, request.Operation)
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, contextError(err)
		}
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI user encryption key source is unavailable")}
	}
	if !bytes.Equal(keyTarget.MAC, target.MAC) {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI user key source changed the target identity")}
	}
	encrypted, err := EncryptISAPIUserField(scheme, key, request.Password)
	if err != nil {
		return result, err
	}
	publicRequest := request
	publicRequest.Password = ""
	method, body, err := w.Profile.BuildUserMutation(publicRequest, encrypted)
	if err != nil {
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI user schema does not support this request")}
	}
	if (request.Operation == ISAPIUserUpdate && method != http.MethodPut) ||
		(request.Operation == ISAPIUserCreate && method != http.MethodPut && method != http.MethodPost) ||
		len(body) == 0 || len(body) > maxISAPIMutationBody || bytes.Count(body, []byte(encrypted.Ciphertext)) != 1 {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user mutation is invalid or too large")}
	}
	base, err := w.Transport.baseURL(target)
	if err != nil {
		return result, err
	}
	path := "/ISAPI/Security/users"
	if request.Operation == ISAPIUserUpdate {
		path += "/" + request.ID
	}
	endpoint := base.ResolveReference(&url.URL{Path: path})
	query := endpoint.Query()
	query.Set("security", strconv.Itoa(int(encrypted.Security)))
	query.Set("iv", encrypted.IV)
	endpoint.RawQuery = query.Encode()
	response, attempted, err := w.Transport.authenticatedMutation(ctx, method, endpoint, body)
	result.MutationAttempted = attempted
	if err != nil {
		return result, err
	}
	if err := validateISAPIResponseStatus(response); err != nil {
		return result, err
	}
	if _, err := w.Transport.discover(ctx, target); err != nil {
		return result, err
	}
	verifyURL := base.ResolveReference(&url.URL{Path: "/ISAPI/Security/users/" + request.ID})
	state, err := w.Transport.requestWithPolicy(ctx, http.MethodGet, verifyURL, nil, true)
	if err != nil {
		return result, err
	}
	if err := w.Profile.VerifyUserState(publicRequest, state); err != nil {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI user state did not verify")}
	}
	result.Verified = true
	return result, nil
}
