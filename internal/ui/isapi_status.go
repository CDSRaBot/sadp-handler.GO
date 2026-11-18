package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func parseISAPIStatus(args []string) (Command, error) {
	command := ISAPIStatusCommand{}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		option := args[i]
		if seen[option] {
			return nil, errors.New("duplicate ISAPI status option")
		}
		seen[option] = true
		switch option {
		case "--https":
			command.HTTPS = true
		case "--address", "--port":
			if i+1 >= len(args) {
				return nil, errors.New("ISAPI status option requires a value")
			}
			i++
			var err error
			switch option {
			case "--address":
				command.Address, err = netip.ParseAddr(args[i])
			case "--port":
				var port uint64
				port, err = strconv.ParseUint(args[i], 10, 16)
				command.Port = uint16(port)
			}
			if err != nil {
				return nil, errors.New("invalid ISAPI status option")
			}
		default:
			return nil, errors.New("unknown ISAPI status option")
		}
	}
	if !seen["--port"] {
		command.Port = 80
		if command.HTTPS {
			command.Port = 443
		}
	}
	if err := validateISAPIStatus(command); err != nil {
		return nil, err
	}
	return command, nil
}

func validateISAPIStatus(command ISAPIStatusCommand) error {
	if !validISAPIAddress(command.Address) || command.Port == 0 {
		return errors.New("invalid ISAPI status inputs")
	}
	return nil
}

func dispatchISAPIStatus(ctx context.Context, command ISAPIStatusCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateISAPIStatus(command); err != nil {
		return dispatchFailure("isapi-status", FailureInvalidArgument, "invalid ISAPI status command", err)
	}
	if dependencies.ISAPIStatus == nil {
		return dispatchUnsupported("isapi-status")
	}
	result, err := dependencies.ISAPIStatus.ReadISAPIStatus(ctx, command)
	if err != nil {
		return dispatchFailure("isapi-status", fault.Classify(err), "ISAPI status failed", err)
	}
	if result.Address != command.Address.String() || result.IdentityVerified ||
		(result.State != "active" && result.State != "inactive") {
		return dispatchFailure("isapi-status", fault.MalformedResponse, "invalid ISAPI status result", errors.New("invalid ISAPI status result"))
	}
	return CommandResult{Command: "isapi-status", OK: true, ISAPIStatus: &result}, nil
}

func formatISAPIStatus(status ISAPIStatusResult) string {
	return fmt.Sprintf("%s state=%s identity=unverified", status.Address, status.State)
}

type ISAPIKnownAddressStatusService struct {
	Client *http.Client
}

func (s ISAPIKnownAddressStatusService) ReadISAPIStatus(ctx context.Context, command ISAPIStatusCommand) (ISAPIStatusResult, error) {
	if ctx == nil {
		return ISAPIStatusResult{}, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("context is required")}
	}
	if err := validateISAPIStatus(command); err != nil {
		return ISAPIStatusResult{}, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	transport := knownAddressTransport(command.Port, command.HTTPS, s.Client)
	target := route.Target{Address: command.Address}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return ISAPIStatusResult{}, err
	}
	state, err := transport.ReadActivationState(ctx, target)
	if err != nil {
		return ISAPIStatusResult{}, err
	}
	result := ISAPIStatusResult{Address: target.Address.String()}
	switch state {
	case route.ISAPIActivationActive:
		result.State = "active"
	case route.ISAPIActivationInactive:
		result.State = "inactive"
	default:
		return ISAPIStatusResult{}, &route.Error{Kind: route.ErrorMalformed, Err: errors.New("invalid ISAPI activation state")}
	}
	return result, nil
}
