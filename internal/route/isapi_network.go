package route

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

// ISAPINetworkProfile must be backed by a verified device-specific schema.
type ISAPINetworkProfile interface {
	BuildUpdate(collection, current []byte, interfaceID string, plan logic.NetworkConfig) ([]byte, error)
	RestartRequired(response []byte) (bool, error)
	Verify(current []byte, plan logic.NetworkConfig) error
}

// ISAPINetworkLocator finds the same device after its address may have changed.
type ISAPINetworkLocator interface {
	Await(context.Context, Target, logic.NetworkConfig) (Target, error)
}

type ISAPINetworkWorkflow struct {
	Transport *ISAPIHTTPTransport
	Profile   ISAPINetworkProfile
	Locator   ISAPINetworkLocator
}

type ISAPINetworkResult struct {
	Original          Target
	Requested         logic.NetworkConfig
	MutationAttempted bool
	RestartRequired   bool
	ObservedAddress   netip.Addr
	Verified          bool
}

func (w ISAPINetworkWorkflow) Configure(ctx context.Context, target Target, interfaceID string, plan logic.NetworkConfig) (ISAPINetworkResult, error) {
	result := ISAPINetworkResult{Original: target, Requested: plan}
	if ctx == nil || w.Transport == nil || w.Profile == nil || w.Locator == nil ||
		w.Transport.Username == "" || w.Transport.Password == "" || len(target.MAC) != 6 ||
		target.Interface != "" || !validISAPIInterfaceID(interfaceID) {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI network workflow arguments are incomplete")}
	}
	if err := logic.ValidateNetworkConfig(plan); err != nil {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI network plan is invalid")}
	}
	if !plan.UseDHCP && (!plan.Address.Addr().IsGlobalUnicast() || !plan.Gateway.IsGlobalUnicast() ||
		!plan.Address.Contains(plan.Gateway) || plan.Address.Addr() == plan.Gateway) {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI static network plan is inconsistent")}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, err := w.Transport.discover(ctx, target); err != nil {
		return result, err
	}
	base, err := w.Transport.baseURL(target)
	if err != nil {
		return result, err
	}
	collectionURL := base.ResolveReference(&url.URL{Path: "/ISAPI/System/Network/interfaces"})
	interfaceURL := base.ResolveReference(&url.URL{Path: "/ISAPI/System/Network/interfaces/" + interfaceID})
	collection, err := w.Transport.requestWithPolicy(ctx, http.MethodGet, collectionURL, nil, true)
	if err != nil {
		return result, err
	}
	current, err := w.Transport.requestWithPolicy(ctx, http.MethodGet, interfaceURL, nil, true)
	if err != nil {
		return result, err
	}
	update, err := w.Profile.BuildUpdate(collection, current, interfaceID, plan)
	if err != nil {
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI network profile does not support this device or plan")}
	}
	if len(update) == 0 || len(update) > maxISAPIMutationBody {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI network update is empty or too large")}
	}
	response, sent, err := w.Transport.authenticatedMutation(ctx, http.MethodPut, interfaceURL, update)
	result.MutationAttempted = sent
	if err != nil {
		return result, err
	}
	if err := validateISAPIResponseStatus(response); err != nil {
		return result, err
	}
	result.RestartRequired, err = w.Profile.RestartRequired(response)
	if err != nil {
		return result, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI network restart status is invalid")}
	}
	if result.RestartRequired {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI network change requires a device restart")}
	}
	observed, err := w.Locator.Await(ctx, target, plan)
	if err != nil {
		if ctx.Err() != nil {
			return result, contextError(ctx.Err())
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, contextError(err)
		}
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI device did not reappear")}
	}
	if !observed.Address.IsValid() || !observed.Address.Is4() || !observed.Address.IsGlobalUnicast() ||
		!bytes.Equal(observed.MAC, target.MAC) || (!plan.UseDHCP && observed.Address != plan.Address.Addr()) ||
		observed.Interface != "" {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI device reappearance identity or address did not match")}
	}
	result.ObservedAddress = observed.Address
	if _, err := w.Transport.discover(ctx, observed); err != nil {
		return result, err
	}
	newBase, err := w.Transport.baseURL(observed)
	if err != nil {
		return result, err
	}
	newURL := newBase.ResolveReference(&url.URL{Path: "/ISAPI/System/Network/interfaces/" + interfaceID})
	verified, err := w.Transport.requestWithPolicy(ctx, http.MethodGet, newURL, nil, true)
	if err != nil {
		return result, err
	}
	if err := w.Profile.Verify(verified, plan); err != nil {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI network configuration did not verify")}
	}
	result.Verified = true
	return result, nil
}

func validISAPIInterfaceID(value string) bool {
	if len(value) == 0 || len(value) > 64 || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '.' || character == '-' || character == '_') {
			return false
		}
	}
	return true
}
