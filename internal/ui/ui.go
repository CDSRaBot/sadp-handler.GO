package ui

import (
	"context"
	"errors"
	"net"
	"time"

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

type Dependencies struct{}

type InterfaceMode string

const (
	ModeTUI            InterfaceMode = "tui"
	ModeCLIInteractive InterfaceMode = "cli-interactive"
	ModeCLIOnce        InterfaceMode = "cli-once"
)

func ParseCLICommand(args []string) (Command, error) {
	return nil, errors.New("not implemented")
}

func TokenizeCLIInput(line string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func ParseConsoleCommand(line string) (Command, error) {
	return nil, errors.New("not implemented")
}

func SelectUIMode(value string, once, stdinIsTerminal, stdoutIsTerminal bool) (InterfaceMode, error) {
	return "", errors.New("not implemented")
}

func SearchDevices(devices []parser.Device, query string) []parser.Device {
	return nil
}

func DispatchCommand(ctx context.Context, command Command, dependencies Dependencies) (CommandResult, error) {
	return CommandResult{}, errors.New("not implemented")
}
