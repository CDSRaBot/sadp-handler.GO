package ui

import (
	"context"
	"errors"
	"net/http"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func parseISAPITerminal(args []string) (Command, error) {
	command := ISAPITerminalCommand{}
	connection := []string{"isapi-discover"}
	seenID := false
	for i := 1; i < len(args); i++ {
		option := args[i]
		if option == "--terminal-id" {
			if seenID || i+1 >= len(args) {
				return nil, errors.New("invalid ISAPI terminal ID option")
			}
			seenID = true
			i++
			command.TerminalID = args[i]
			continue
		}
		connection = append(connection, option)
		if option == "--address" || option == "--device" || option == "--username" || option == "--port" {
			if i+1 >= len(args) {
				return nil, errors.New("ISAPI connection option requires a value")
			}
			i++
			connection = append(connection, args[i])
		}
	}
	parsed, err := parseISAPIDiscover(connection)
	if err != nil {
		return nil, err
	}
	command.Connection = parsed.(ISAPIDiscoverCommand)
	if err := validateISAPITerminal(command); err != nil {
		return nil, err
	}
	return command, nil
}

func validateISAPITerminal(command ISAPITerminalCommand) error {
	if err := validateISAPIDiscover(command.Connection); err != nil {
		return err
	}
	return route.ValidateISAPITerminalID(command.TerminalID)
}

func dispatchISAPITerminal(ctx context.Context, command ISAPITerminalCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateISAPITerminal(command); err != nil {
		return dispatchFailure("isapi-terminal", FailureInvalidArgument, "invalid ISAPI terminal command", err)
	}
	if !dependencies.Config.Once || dependencies.Config.Mode != "no" {
		return dispatchFailure("isapi-terminal", FailureUnsupported, "ISAPI terminal status requires one-shot CLI mode", errors.New("ISAPI terminal status requires one-shot CLI mode"))
	}
	if dependencies.ISAPITerminal == nil {
		return dispatchUnsupported("isapi-terminal")
	}
	result, err := dependencies.ISAPITerminal.ReadISAPITerminal(ctx, command)
	if err != nil {
		for _, inputErr := range []error{errPasswordInputUnavailable, errPasswordInputTerminal, errPasswordInputRead, errPasswordInputInvalid} {
			if errors.Is(err, inputErr) {
				return dispatchFailure("isapi-terminal", FailureInvalidArgument, inputErr.Error(), err)
			}
		}
		return dispatchFailure("isapi-terminal", fault.Classify(err), "ISAPI terminal status failed", err)
	}
	return CommandResult{Command: "isapi-terminal", OK: true, ISAPITerminal: &result}, nil
}

type ISAPIKnownAddressTerminalService struct {
	Passwords PasswordProvider
	Client    *http.Client
}

func (s ISAPIKnownAddressTerminalService) ReadISAPITerminal(ctx context.Context, command ISAPITerminalCommand) (ISAPITerminalResult, error) {
	if ctx == nil || s.Passwords == nil {
		return ISAPITerminalResult{}, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI terminal dependencies are incomplete")}
	}
	if err := validateISAPITerminal(command); err != nil {
		return ISAPITerminalResult{}, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	connection := command.Connection
	transport := knownAddressTransport(connection.Port, connection.HTTPS, s.Client)
	transport.Username = connection.Username
	transport.AllowBasic = connection.AllowBasic
	target := route.Target{MAC: connection.MAC, Address: connection.Address}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return ISAPITerminalResult{}, err
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return ISAPITerminalResult{}, err
	}
	transport.Password = password
	discovery, err := transport.ReadTerminalDiscovery(ctx, target, command.TerminalID)
	if err != nil {
		return ISAPITerminalResult{}, err
	}
	return ISAPITerminalResult{
		Address: connection.Address.String(), MAC: connection.MAC.String(),
		TerminalID: command.TerminalID, Discovery: discovery,
	}, nil
}
