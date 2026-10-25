package ui

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/i18n"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

type Command interface{ isCommand() }

type DiscoverCommand struct {
	InterfaceIndex *int
	Policy         DiscoveryPolicy
}

type DiscoveryPolicy struct {
	ProbeInterval       time.Duration
	CycleTimeout        time.Duration
	RepeatCount         int
	MaxSADPDatagramSize int
}

type ActivateCommand struct {
	Targets       TargetSelector
	PasswordInput PasswordSource
	Email         string
	Confirm       bool
}

type NetworkCommand struct {
	Targets       TargetSelector
	PasswordInput PasswordSource
	Config        logic.NetworkConfig
	Confirm       bool
}

type LanguageCommand struct {
	Action LanguageAction
	Code   string
}

type HelpCommand struct{}
type VersionCommand struct{}
type QuitCommand struct{}

type ExportLocaleTemplateCommand struct {
	Destination string
}

func (DiscoverCommand) isCommand()             {}
func (ActivateCommand) isCommand()             {}
func (NetworkCommand) isCommand()              {}
func (LanguageCommand) isCommand()             {}
func (HelpCommand) isCommand()                 {}
func (VersionCommand) isCommand()              {}
func (QuitCommand) isCommand()                 {}
func (ExportLocaleTemplateCommand) isCommand() {}

type TargetSelector struct {
	MACs             []net.HardwareAddr
	UseTUISelection  bool
	UseFocusedDevice bool
}

type PasswordSource uint8

const (
	PasswordFromPrompt PasswordSource = iota + 1
	PasswordFromStdin
)

type LanguageAction uint8

const (
	LanguageList LanguageAction = iota + 1
	LanguageSwitch
)

type CommandResult struct {
	Command   string             `json:"command,omitempty"`
	OK        bool               `json:"ok"`
	Exit      bool               `json:"exit,omitempty"`
	Message   string             `json:"message,omitempty"`
	Discovery []DiscoveredDevice `json:"discovery,omitempty"`
	Devices   []DeviceOutcome    `json:"devices,omitempty"`
	Locales   []string           `json:"locales,omitempty"`
	Version   string             `json:"version,omitempty"`
	Error     *CommandFailure    `json:"error,omitempty"`
}

type DiscoveredDevice struct {
	MAC             string    `json:"mac"`
	Sender          string    `json:"sender"`
	IPv4            string    `json:"ipv4"`
	Model           string    `json:"model,omitempty"`
	Activated       bool      `json:"activated"`
	SoftwareVersion string    `json:"software_version,omitempty"`
	LastSeen        time.Time `json:"last_seen"`
}

type DeviceOutcome struct {
	MAC    string `json:"mac"`
	Sender string `json:"sender,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type CommandFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	FailureInvalidArgument = "invalid_argument"
	FailureUnsupported     = "unsupported"
	FailureOperationFailed = "operation_failed"
	FailureCanceled        = "canceled"
)

type DiscoveryService interface {
	Discover(context.Context, *int, DiscoveryPolicy) ([]DiscoveredDevice, error)
}

type DiscoverySession interface {
	Events() <-chan DiscoveredDevice
	Errors() <-chan error
}

type DiscoverySessionService interface {
	Start(context.Context, *int, DiscoveryPolicy) (DiscoverySession, error)
}

type InterfaceLister interface {
	ListEligibleInterfaces() ([]adapter.Interface, error)
}

type LocalizationService interface {
	ListLocales(context.Context) ([]string, error)
	SwitchLocale(context.Context, string) error
	ExportLocaleTemplate(context.Context, string) error
}

type ActivationService interface {
	Activate(context.Context, ActivateCommand) ([]DeviceOutcome, error)
}

type NetworkService interface {
	ConfigureNetwork(context.Context, NetworkCommand) ([]DeviceOutcome, error)
}

type RuntimeUIConfig struct {
	Mode     string
	Format   string
	Language string
	Once     bool
}

type Dependencies struct {
	Config           RuntimeUIConfig
	Discovery        DiscoveryService
	DiscoverySession DiscoverySessionService
	InterfaceLister  InterfaceLister
	InterfaceIndex   *int
	DiscoveryPolicy  DiscoveryPolicy
	Locales          LocalizationService
	Activation       ActivationService
	Network          NetworkService
	Version          string
	Help             string
}

type InterfaceMode string

const (
	ModeTUI            InterfaceMode = "tui"
	ModeCLIInteractive InterfaceMode = "cli-interactive"
	ModeCLIOnce        InterfaceMode = "cli-once"
)

func SelectUIMode(value string, once, stdinIsTerminal, stdoutIsTerminal bool) (InterfaceMode, error) {
	switch value {
	case "yes":
		if once {
			return "", errors.New("--once requires --ui=no")
		}
		if !stdinIsTerminal || !stdoutIsTerminal {
			return "", errors.New("TUI requires terminal input and output; use --ui=no")
		}
		return ModeTUI, nil
	case "no":
		if once {
			return ModeCLIOnce, nil
		}
		if !stdinIsTerminal {
			return "", errors.New("interactive CLI requires terminal input; use --once")
		}
		return ModeCLIInteractive, nil
	default:
		return "", errors.New("--ui must be yes or no")
	}
}

func SearchDevices(devices []parser.Device, query string) []parser.Device {
	normalized := strings.ToLower(strings.TrimSpace(query))
	result := make([]parser.Device, 0, len(devices))
	for _, device := range devices {
		if normalized == "" || deviceMatches(device, normalized) {
			result = append(result, device)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return !result[i].Activated && result[j].Activated
	})
	return result
}

func deviceMatches(device parser.Device, query string) bool {
	values := []string{device.MAC.String(), device.IPv4.String(), device.SerialNumber, device.Model}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func DispatchCommand(ctx context.Context, command Command, dependencies Dependencies) (CommandResult, error) {
	if ctx == nil {
		return dispatchFailure("", FailureInvalidArgument, "context is nil", errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return dispatchFailure(commandName(command), FailureCanceled, "command canceled", err)
	}
	if command == nil {
		return dispatchFailure("", FailureInvalidArgument, "command is nil", errors.New("command is nil"))
	}

	switch value := command.(type) {
	case DiscoverCommand:
		return dispatchDiscover(ctx, value, dependencies)
	case ActivateCommand:
		return dispatchActivate(ctx, value, dependencies)
	case NetworkCommand:
		return dispatchNetwork(ctx, value, dependencies)
	case LanguageCommand:
		return dispatchLanguage(ctx, value, dependencies)
	case HelpCommand:
		message := dependencies.Help
		if message == "" {
			message = "discover, activate, network, language, help, version, quit"
		}
		return CommandResult{Command: "help", OK: true, Message: message}, nil
	case VersionCommand:
		version := dependencies.Version
		if version == "" {
			version = "dev"
		}
		return CommandResult{Command: "version", OK: true, Version: version, Message: version}, nil
	case QuitCommand:
		return CommandResult{Command: "quit", OK: true, Exit: true}, nil
	case ExportLocaleTemplateCommand:
		if dependencies.Locales == nil {
			return dispatchFailure("export-locale-template", FailureOperationFailed, "localization service is unavailable", errors.New("localization service is unavailable"))
		}
		if err := dependencies.Locales.ExportLocaleTemplate(ctx, value.Destination); err != nil {
			if isCancellation(err) {
				return dispatchFailure("export-locale-template", FailureCanceled, "command canceled", err)
			}
			return dispatchFailure("export-locale-template", FailureOperationFailed, "locale template export failed", err)
		}
		return CommandResult{Command: "export-locale-template", OK: true}, nil
	default:
		return dispatchFailure("", FailureInvalidArgument, "unsupported command type", errors.New("unsupported command type"))
	}
}

func dispatchDiscover(ctx context.Context, command DiscoverCommand, dependencies Dependencies) (CommandResult, error) {
	if dependencies.Discovery == nil {
		return dispatchFailure("discover", FailureOperationFailed, "discovery service is unavailable", errors.New("discovery service is unavailable"))
	}

	if err := validateDiscoveryPolicy(command.Policy); err != nil {
		return dispatchFailure("discover", FailureInvalidArgument, "invalid discovery policy", err)
	}
	devices, err := dependencies.Discovery.Discover(ctx, command.InterfaceIndex, command.Policy)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return dispatchFailure("discover", FailureCanceled, "command canceled", err)
		}
		return dispatchFailure("discover", FailureOperationFailed, "discovery failed", err)
	}
	return CommandResult{Command: "discover", OK: true, Discovery: devices}, nil
}

func dispatchActivate(ctx context.Context, command ActivateCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateMutationCommand(command.Targets, command.PasswordInput, command.Confirm); err != nil {
		return dispatchFailure("activate", FailureInvalidArgument, "invalid activation command", err)
	}
	if dependencies.Activation == nil {
		return dispatchUnsupported("activate")
	}
	outcomes, err := dependencies.Activation.Activate(ctx, command)
	return dispatchMutationResult(ctx, "activate", outcomes, err)
}

func dispatchNetwork(ctx context.Context, command NetworkCommand, dependencies Dependencies) (CommandResult, error) {
	if err := validateMutationCommand(command.Targets, command.PasswordInput, command.Confirm); err != nil {
		return dispatchFailure("network", FailureInvalidArgument, "invalid network command", err)
	}
	if dependencies.Network == nil {
		return dispatchUnsupported("network")
	}
	outcomes, err := dependencies.Network.ConfigureNetwork(ctx, command)
	return dispatchMutationResult(ctx, "network", outcomes, err)
}

func validateMutationCommand(targets TargetSelector, password PasswordSource, confirmed bool) error {
	if password != PasswordFromStdin {
		return errors.New("password must be supplied through stdin")
	}
	if len(targets.MACs) == 0 {
		return errors.New("at least one target device is required")
	}
	if len(targets.MACs) > 1 && !confirmed {
		return errors.New("bulk mutation requires confirmation")
	}
	seen := make(map[string]struct{}, len(targets.MACs))
	for _, mac := range targets.MACs {
		if len(mac) != 6 {
			return errors.New("target MAC must be an EUI-48 address")
		}
		key := mac.String()
		if _, ok := seen[key]; ok {
			return errors.New("duplicate target device")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func dispatchMutationResult(ctx context.Context, command string, outcomes []DeviceOutcome, err error) (CommandResult, error) {
	if err != nil {
		if isCancellation(err) {
			return dispatchFailure(command, FailureCanceled, "command canceled", err)
		}
		var operationErr *logic.DeviceOperationError
		if errors.As(err, &operationErr) && operationErr.Kind == logic.OperationInvalidInput {
			return dispatchFailure(command, FailureInvalidArgument, "invalid device operation", err)
		}
		return dispatchFailure(command, FailureOperationFailed, command+" operation failed", err)
	}
	return CommandResult{Command: command, OK: true, Devices: outcomes}, nil
}

func dispatchLanguage(ctx context.Context, command LanguageCommand, dependencies Dependencies) (CommandResult, error) {
	if dependencies.Locales == nil {
		return dispatchFailure("language", FailureOperationFailed, "localization service is unavailable", errors.New("localization service is unavailable"))
	}
	switch command.Action {
	case LanguageList:
		locales, err := dependencies.Locales.ListLocales(ctx)
		if err != nil {
			if errors.Is(err, i18n.ErrNoExternalLocales) {
				return CommandResult{Command: "language", OK: true, Locales: locales}, nil
			}
			if isCancellation(err) {
				return dispatchFailure("language", FailureCanceled, "command canceled", err)
			}
			return dispatchFailure("language", FailureOperationFailed, "locale listing failed", err)
		}
		return CommandResult{Command: "language", OK: true, Locales: locales}, nil
	case LanguageSwitch:
		if strings.TrimSpace(command.Code) == "" {
			return dispatchFailure("language", FailureInvalidArgument, "locale code is required", errors.New("locale code is required"))
		}
		if err := dependencies.Locales.SwitchLocale(ctx, command.Code); err != nil {
			if isCancellation(err) {
				return dispatchFailure("language", FailureCanceled, "command canceled", err)
			}
			return dispatchFailure("language", FailureOperationFailed, "locale switch failed", err)
		}
		return CommandResult{Command: "language", OK: true, Message: command.Code}, nil
	default:
		return dispatchFailure("language", FailureInvalidArgument, "invalid language action", errors.New("invalid language action"))
	}
}

func dispatchUnsupported(command string) (CommandResult, error) {
	return dispatchFailure(command, FailureUnsupported, command+" workflow is not implemented", errors.New("workflow is not implemented"))
}

func dispatchFailure(command, code, message string, cause error) (CommandResult, error) {
	result := CommandResult{
		Command: command,
		OK:      false,
		Error:   &CommandFailure{Code: code, Message: message},
	}
	if cause == nil {
		cause = errors.New(message)
	}
	return result, cause
}

func commandName(command Command) string {
	switch command.(type) {
	case DiscoverCommand:
		return "discover"
	case ActivateCommand:
		return "activate"
	case NetworkCommand:
		return "network"
	case LanguageCommand:
		return "language"
	case HelpCommand:
		return "help"
	case VersionCommand:
		return "version"
	case QuitCommand:
		return "quit"
	case ExportLocaleTemplateCommand:
		return "export-locale-template"
	default:
		return ""
	}
}

func validateDiscoveryPolicy(policy DiscoveryPolicy) error {
	if policy.MaxSADPDatagramSize < 1 || policy.MaxSADPDatagramSize > 65507 {
		return errors.New("maximum datagram size must be between 1 and 65507")
	}
	continuous := policy.ProbeInterval > 0 && policy.CycleTimeout == 0 && policy.RepeatCount == 0
	bounded := policy.ProbeInterval == 0 && policy.CycleTimeout > 0 && policy.RepeatCount >= 1 && policy.RepeatCount <= 100
	if !continuous && !bounded {
		return errors.New("discovery policy must be continuous or bounded")
	}
	return nil
}

func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
