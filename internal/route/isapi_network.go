package route

import (
	"bytes"
	"context"
	"encoding/binary"
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

// ISAPINetworkDeviceGate verifies an authenticated device against a specific
// network profile before the workflow reads or writes network configuration.
type ISAPINetworkDeviceGate interface {
	VerifyNetworkDevice(context.Context, Target, DeviceInfo) error
}

type ISAPINetworkWorkflow struct {
	Transport *ISAPIHTTPTransport
	Profile   ISAPINetworkProfile
	Gate      ISAPINetworkDeviceGate
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
	if ctx == nil || w.Transport == nil || w.Profile == nil || w.Gate == nil || w.Locator == nil ||
		w.Transport.Username == "" || w.Transport.Password == "" || len(target.MAC) != 6 ||
		target.Interface != "" {
		return result, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI network workflow arguments are incomplete")}
	}
	if err := ValidateISAPINetworkInputs(interfaceID, plan); err != nil {
		return result, err
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
	if err := w.Gate.VerifyNetworkDevice(ctx, gateTarget, *discovered.Device); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, contextError(err)
		}
		var routeErr *Error
		if errors.As(err, &routeErr) {
			return result, err
		}
		return result, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI network profile is not verified for this device")}
	}
	if !bytes.Equal(gateTarget.MAC, target.MAC) {
		return result, &Error{Kind: ErrorVerification, Err: errors.New("ISAPI network profile gate changed the target identity")}
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

// ValidateISAPINetworkInputs checks the interface identifier and network plan.
func ValidateISAPINetworkInputs(interfaceID string, plan logic.NetworkConfig) error {
	if !validISAPIInterfaceID(interfaceID) || !validISAPINetworkPlan(plan) {
		return &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI network interface or plan is invalid")}
	}
	return nil
}

func validISAPINetworkPlan(plan logic.NetworkConfig) bool {
	if logic.ValidateNetworkConfig(plan) != nil {
		return false
	}
	if plan.UseDHCP {
		return true
	}
	if !plan.Address.Addr().IsGlobalUnicast() || !plan.Gateway.IsGlobalUnicast() ||
		!plan.Address.Contains(plan.Gateway) || plan.Address.Addr() == plan.Gateway {
		return false
	}
	if plan.Address.Bits() <= 30 {
		network := plan.Address.Masked().Addr().As4()
		address := plan.Address.Addr().As4()
		gateway := plan.Gateway.As4()
		first := binary.BigEndian.Uint32(network[:])
		last := first | (^uint32(0) >> uint(plan.Address.Bits()))
		ip := binary.BigEndian.Uint32(address[:])
		gw := binary.BigEndian.Uint32(gateway[:])
		if ip == first || ip == last || gw == first || gw == last {
			return false
		}
	}
	return true
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
