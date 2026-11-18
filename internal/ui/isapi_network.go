package ui

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func parseISAPINetwork(args []string) (Command, error) {
	command := ISAPINetworkCommand{}
	connection := []string{"isapi-discover"}
	seen := make(map[string]bool)
	for i := 1; i < len(args); i++ {
		option := args[i]
		switch option {
		case "--dhcp", "--yes":
			if seen[option] {
				return nil, errors.New("duplicate ISAPI network option")
			}
			seen[option] = true
			if option == "--dhcp" {
				command.Plan.UseDHCP = true
			} else {
				command.Confirm = true
			}
		case "--interface-id", "--discovery-interface", "--new-address", "--gateway":
			if seen[option] || i+1 >= len(args) {
				return nil, errors.New("invalid ISAPI network option")
			}
			seen[option] = true
			i++
			var err error
			switch option {
			case "--interface-id":
				command.InterfaceID = args[i]
			case "--discovery-interface":
				command.DiscoveryInterface, err = strconv.Atoi(args[i])
			case "--new-address":
				command.Plan.Address, err = netip.ParsePrefix(args[i])
			case "--gateway":
				command.Plan.Gateway, err = netip.ParseAddr(args[i])
			}
			if err != nil {
				return nil, errors.New("invalid ISAPI network option value")
			}
		default:
			connection = append(connection, option)
			if option == "--address" || option == "--device" || option == "--username" || option == "--port" {
				if i+1 >= len(args) {
					return nil, errors.New("ISAPI connection option requires a value")
				}
				i++
				connection = append(connection, args[i])
			}
		}
	}
	parsed, err := parseISAPIDiscover(connection)
	if err != nil {
		return nil, err
	}
	command.Connection = parsed.(ISAPIDiscoverCommand)
	if seen["--discovery-interface"] && !command.Plan.UseDHCP {
		return nil, errors.New("discovery interface is only valid for DHCP")
	}
	if err := validateISAPINetwork(command); err != nil {
		return nil, err
	}
	return command, nil
}

func validateISAPINetwork(command ISAPINetworkCommand) error {
	if err := validateISAPIDiscover(command.Connection); err != nil {
		return err
	}
	if !command.Confirm || (command.Plan.UseDHCP && command.DiscoveryInterface < 1) ||
		(!command.Plan.UseDHCP && command.DiscoveryInterface != 0) {
		return errors.New("ISAPI network requires confirmation and a DHCP discovery interface when applicable")
	}
	return route.ValidateISAPINetworkInputs(command.InterfaceID, command.Plan)
}

func dispatchISAPINetwork(ctx context.Context, command ISAPINetworkCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateISAPINetwork(command); err != nil {
		return dispatchFailure("isapi-network", FailureInvalidArgument, "invalid ISAPI network command", err)
	}
	if !dependencies.Config.Once || dependencies.Config.Mode != "no" {
		return dispatchFailure("isapi-network", FailureUnsupported, "ISAPI network requires one-shot CLI mode", errors.New("ISAPI network requires one-shot CLI mode"))
	}
	if dependencies.ISAPINetwork == nil {
		return dispatchUnsupported("isapi-network")
	}
	network, err := dependencies.ISAPINetwork.ConfigureISAPINetwork(ctx, command)
	public := ISAPINetworkResult{
		Address: command.Connection.Address.String(), MAC: command.Connection.MAC.String(),
		Attempted: network.Attempted, RestartRequired: network.RestartRequired,
	}
	if network.ObservedAddress != "" {
		address, parseErr := netip.ParseAddr(network.ObservedAddress)
		if parseErr != nil || !address.Is4() || !address.IsGlobalUnicast() {
			result, dispatchErr := dispatchFailure("isapi-network", fault.MalformedResponse, "invalid ISAPI network result", errors.New("invalid observed address"))
			result.ISAPINetwork = &public
			return result, dispatchErr
		}
		public.ObservedAddress = address.String()
	}
	if err != nil {
		code := fault.Classify(err)
		message := "ISAPI network change failed; if attempted, the device state is unknown"
		for _, inputErr := range []error{errPasswordInputUnavailable, errPasswordInputTerminal, errPasswordInputRead, errPasswordInputInvalid} {
			if errors.Is(err, inputErr) {
				code = FailureInvalidArgument
				message = inputErr.Error()
				break
			}
		}
		if code == FailureUnsupported && !public.Attempted {
			message = "ISAPI network change is disabled: a verified device profile is required"
		}
		if public.RestartRequired {
			message = "ISAPI network change requires a device restart; configuration is not verified"
		}
		result, dispatchErr := dispatchFailure("isapi-network", code, message, err)
		result.ISAPINetwork = &public
		return result, dispatchErr
	}
	if network.Address != public.Address || network.MAC != public.MAC || !network.Attempted || !network.Verified ||
		network.RestartRequired || public.ObservedAddress == "" ||
		(!command.Plan.UseDHCP && public.ObservedAddress != command.Plan.Address.Addr().String()) {
		result, dispatchErr := dispatchFailure("isapi-network", fault.MalformedResponse, "invalid ISAPI network result", errors.New("invalid ISAPI network result"))
		result.ISAPINetwork = &public
		return result, dispatchErr
	}
	public.Verified = true
	return CommandResult{Command: "isapi-network", OK: true, ISAPINetwork: &public}, nil
}

type ISAPIKnownAddressNetworkService struct {
	Passwords  PasswordProvider
	Client     *http.Client
	Profile    route.ISAPINetworkProfile
	Gate       route.ISAPINetworkDeviceGate
	Sessions   DiscoverySessionService
	Interfaces InterfaceLister
	Policy     DiscoveryPolicy
}

func (s ISAPIKnownAddressNetworkService) ConfigureISAPINetwork(ctx context.Context, command ISAPINetworkCommand) (ISAPINetworkResult, error) {
	connection := command.Connection
	result := ISAPINetworkResult{Address: connection.Address.String(), MAC: connection.MAC.String()}
	if ctx == nil || s.Passwords == nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI network dependencies are incomplete")}
	}
	if err := validateISAPINetwork(command); err != nil {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s.Profile == nil || s.Gate == nil {
		return result, &route.Error{Kind: route.ErrorUnsupported, Err: errors.New("verified ISAPI network profile is required")}
	}
	if command.Plan.UseDHCP && (s.Sessions == nil || s.Interfaces == nil || s.Policy.ProbeInterval <= 0 || validateDiscoveryPolicy(s.Policy) != nil) {
		return result, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("DHCP discovery dependencies are incomplete")}
	}
	if command.Plan.UseDHCP {
		interfaces, err := s.Interfaces.ListEligibleInterfaces()
		if err != nil {
			return result, &route.Error{Kind: route.ErrorTransport, Err: err}
		}
		found := false
		for _, iface := range interfaces {
			if iface.Index == command.DiscoveryInterface {
				found = true
				break
			}
		}
		if !found {
			return result, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("DHCP discovery interface is not eligible")}
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	transport := knownAddressTransport(connection.Port, connection.HTTPS, s.Client)
	transport.Username = connection.Username
	transport.AllowBasic = connection.AllowBasic
	target := route.Target{Address: connection.Address, MAC: append([]byte(nil), connection.MAC...)}
	if err := checkKnownAddressRoute(ctx, transport, target); err != nil {
		return result, err
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return result, err
	}
	transport.Password = password
	var locator route.ISAPINetworkLocator = route.ISAPIStaticNetworkLocator{Transport: transport}
	if command.Plan.UseDHCP {
		locator = ISAPIDHCPNetworkLocator{
			Sessions: s.Sessions, InterfaceIndex: command.DiscoveryInterface,
			Policy: s.Policy, Transport: transport,
		}
	}
	workflow := route.ISAPINetworkWorkflow{Transport: transport, Profile: s.Profile, Gate: s.Gate, Locator: locator}
	operation, err := workflow.Configure(ctx, target, command.InterfaceID, command.Plan)
	result.Attempted = operation.MutationAttempted
	result.RestartRequired = operation.RestartRequired
	result.Verified = operation.Verified
	if operation.ObservedAddress.IsValid() {
		result.ObservedAddress = operation.ObservedAddress.String()
	}
	return result, err
}
