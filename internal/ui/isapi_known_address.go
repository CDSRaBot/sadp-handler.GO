package ui

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func parseISAPIDiscover(args []string) (Command, error) {
	command := ISAPIDiscoverCommand{}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		option := args[i]
		if seen[option] {
			return nil, errors.New("duplicate ISAPI discovery option")
		}
		seen[option] = true
		switch option {
		case "--https":
			command.HTTPS = true
		case "--basic":
			command.AllowBasic = true
		case "--password-stdin":
			command.PasswordInput = PasswordFromStdin
		case "--address", "--device", "--username", "--port":
			if i+1 >= len(args) {
				return nil, errors.New("ISAPI discovery option requires a value")
			}
			i++
			value := args[i]
			var err error
			switch option {
			case "--address":
				command.Address, err = netip.ParseAddr(value)
			case "--device":
				command.MAC, err = parser.NormalizeMAC(value)
			case "--username":
				command.Username = value
			case "--port":
				var port uint64
				port, err = strconv.ParseUint(value, 10, 16)
				command.Port = uint16(port)
			}
			if err != nil {
				return nil, errors.New("invalid ISAPI discovery option")
			}
		default:
			return nil, errors.New("unknown ISAPI discovery option")
		}
	}
	if !seen["--port"] {
		command.Port = 80
		if command.HTTPS {
			command.Port = 443
		}
	}
	if err := validateISAPIDiscover(command); err != nil {
		return nil, err
	}
	return command, nil
}

func validateISAPIDiscover(command ISAPIDiscoverCommand) error {
	if !validISAPIAddress(command.Address) || len(command.MAC) != 6 ||
		command.Port == 0 || command.Username == "" || len(command.Username) > 128 ||
		!utf8.ValidString(command.Username) || strings.ContainsAny(command.Username, "\x00\r\n") ||
		command.PasswordInput != PasswordFromStdin || (command.AllowBasic && !command.HTTPS) {
		return errors.New("invalid ISAPI discovery inputs")
	}
	return nil
}

func validISAPIAddress(address netip.Addr) bool {
	return address.IsValid() && !address.IsUnspecified() && !address.IsMulticast() &&
		address.Zone() == "" && !address.IsLinkLocalUnicast()
}

func dispatchISAPIDiscover(ctx context.Context, command ISAPIDiscoverCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateISAPIDiscover(command); err != nil {
		return dispatchFailure("isapi-discover", FailureInvalidArgument, "invalid ISAPI discovery command", err)
	}
	if !dependencies.Config.Once || dependencies.Config.Mode != "no" {
		return dispatchFailure("isapi-discover", FailureUnsupported, "ISAPI discovery requires one-shot CLI mode", errors.New("ISAPI discovery requires one-shot CLI mode"))
	}
	if dependencies.ISAPIDiscovery == nil {
		return dispatchUnsupported("isapi-discover")
	}
	device, err := dependencies.ISAPIDiscovery.DiscoverISAPI(ctx, command)
	if err != nil {
		for _, inputErr := range []error{errPasswordInputUnavailable, errPasswordInputTerminal, errPasswordInputRead, errPasswordInputInvalid} {
			if errors.Is(err, inputErr) {
				return dispatchFailure("isapi-discover", FailureInvalidArgument, inputErr.Error(), err)
			}
		}
		return dispatchFailure("isapi-discover", fault.Classify(err), "ISAPI discovery failed", err)
	}
	return CommandResult{Command: "isapi-discover", OK: true, ISAPIDevice: &device}, nil
}

type ISAPIKnownAddressService struct {
	Passwords PasswordProvider
	Client    *http.Client
}

func (s ISAPIKnownAddressService) DiscoverISAPI(ctx context.Context, command ISAPIDiscoverCommand) (ISAPIDiscoveredDevice, error) {
	if ctx == nil || s.Passwords == nil {
		return ISAPIDiscoveredDevice{}, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI discovery dependencies are incomplete")}
	}
	if err := validateISAPIDiscover(command); err != nil {
		return ISAPIDiscoveredDevice{}, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	transport := knownAddressTransport(command.Port, command.HTTPS, s.Client)
	transport.Username = command.Username
	transport.AllowBasic = command.AllowBasic
	target := route.Target{MAC: command.MAC, Address: command.Address}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return ISAPIDiscoveredDevice{}, err
	}
	var err error
	transport.Password, err = s.Passwords.Password(ctx)
	if err != nil {
		return ISAPIDiscoveredDevice{}, err
	}

	result, err := transport.Execute(ctx, route.OperationDiscovery, target)
	if err != nil {
		return ISAPIDiscoveredDevice{}, err
	}
	if result.Device == nil {
		return ISAPIDiscoveredDevice{}, &route.Error{Kind: route.ErrorMalformed, Err: errors.New("ISAPI device information is missing")}
	}
	return ISAPIDiscoveredDevice{
		MAC: result.Target.MAC.String(), Address: result.Target.Address.String(),
		Model: result.Device.Model, SerialNumber: result.Device.SerialNumber,
		FirmwareVersion: result.Device.FirmwareVersion,
	}, nil
}

func knownAddressTransport(port uint16, https bool, client *http.Client) *route.ISAPIHTTPTransport {
	scheme := "http"
	if https {
		scheme = "https"
	}
	return &route.ISAPIHTTPTransport{
		BaseURL: scheme + "://" + net.JoinHostPort("device", strconv.Itoa(int(port))),
		Client:  client,
	}
}

func checkKnownAddressRoute(ctx context.Context, transport *route.ISAPIHTTPTransport, target route.Target) error {
	reachable, err := (route.ISAPIRouteProvider{Transport: transport}).CanReach(ctx, target.Address, "")
	if err != nil {
		return err
	}
	if !reachable {
		return &route.Error{Kind: route.ErrorNoRoute, Err: errors.New("ISAPI target is not reachable")}
	}
	return nil
}
