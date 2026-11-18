package ui

import (
	"bytes"
	"context"
	"errors"
	"net/http"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func dispatchISAPIActivate(ctx context.Context, command ISAPIActivateCommand, dependencies Dependencies) (CommandResult, error) {
	discovery := ISAPIDiscoverCommand(command)
	if err := validateISAPIDiscover(discovery); err != nil {
		return dispatchFailure("isapi-activate", FailureInvalidArgument, "invalid ISAPI activation command", err)
	}
	if !dependencies.Config.Once || dependencies.Config.Mode != "no" {
		return dispatchFailure("isapi-activate", FailureUnsupported, "ISAPI activation requires one-shot CLI mode", errors.New("ISAPI activation requires one-shot CLI mode"))
	}
	if dependencies.ISAPIActivation == nil {
		return dispatchUnsupported("isapi-activate")
	}
	activation, err := dependencies.ISAPIActivation.ActivateISAPI(ctx, command)
	public := ISAPIActivationResult{
		Address: discovery.Address.String(), MAC: discovery.MAC.String(), Attempted: activation.Attempted,
	}
	if err != nil {
		code := fault.Classify(err)
		message := "ISAPI activation failed; if attempted, the device state is unknown"
		for _, inputErr := range []error{errPasswordInputUnavailable, errPasswordInputTerminal, errPasswordInputRead, errPasswordInputInvalid} {
			if errors.Is(err, inputErr) {
				code = FailureInvalidArgument
				message = inputErr.Error()
				break
			}
		}
		if code == FailureUnsupported {
			message = "ISAPI activation is disabled: verified device profile and identity are required"
		}
		result, dispatchErr := dispatchFailure("isapi-activate", code, message, err)
		result.ISAPIActivation = &public
		return result, dispatchErr
	}
	if activation.Address != public.Address || activation.MAC != public.MAC || !activation.Attempted || !activation.Verified {
		result, dispatchErr := dispatchFailure("isapi-activate", fault.MalformedResponse, "invalid ISAPI activation result", errors.New("invalid ISAPI activation result"))
		result.ISAPIActivation = &public
		return result, dispatchErr
	}
	public.Verified = true
	return CommandResult{Command: "isapi-activate", OK: true, ISAPIActivation: &public}, nil
}

type ISAPIKnownAddressActivationService struct {
	Passwords PasswordProvider
	Client    *http.Client
	Gate      route.ISAPIActivationGate
}

func (s ISAPIKnownAddressActivationService) ActivateISAPI(ctx context.Context, command ISAPIActivateCommand) (ISAPIActivationResult, error) {
	discovery := ISAPIDiscoverCommand(command)
	result := ISAPIActivationResult{Address: discovery.Address.String(), MAC: discovery.MAC.String()}
	if ctx == nil || s.Passwords == nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI activation dependencies are incomplete")}
	}
	if err := validateISAPIDiscover(discovery); err != nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s.Gate == nil {
		return result, &route.Error{Kind: route.ErrorUnsupported, Err: errors.New("verified ISAPI activation profile and target identity are required")}
	}
	transport := knownAddressTransport(discovery.Port, discovery.HTTPS, s.Client)
	transport.Username = discovery.Username
	transport.AllowBasic = discovery.AllowBasic
	transport.ActivationGate = s.Gate
	transport.Verifier = route.ISAPIDeviceInfoVerifier{Transport: transport}
	target := route.Target{MAC: append([]byte(nil), discovery.MAC...), Address: discovery.Address}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return result, err
	}
	gateTarget := target
	gateTarget.MAC = append([]byte(nil), target.MAC...)
	if err := s.Gate.VerifyActivationTarget(ctx, gateTarget); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, err
		}
		var routeErr *route.Error
		if errors.As(err, &routeErr) {
			return result, err
		}
		return result, &route.Error{Kind: route.ErrorVerification, Err: err}
	}
	if !bytes.Equal(gateTarget.MAC, target.MAC) {
		return result, &route.Error{Kind: route.ErrorVerification, Err: errors.New("ISAPI activation gate changed the target identity")}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return result, err
	}
	transport.Password = password
	operation, err := transport.Execute(ctx, route.OperationActivation, target)
	result.Attempted = operation.ActivationAttempted
	if err != nil {
		return result, err
	}
	result.Verified = true
	return result, nil
}
