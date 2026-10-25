package ui

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func TokenizeCLIInput(line string) ([]string, error) {
	var tokens []string
	var token strings.Builder
	inQuotes := false
	quoted := false
	for i := 0; i < len(line); {
		ch, size := utf8.DecodeRuneInString(line[i:])
		if ch == utf8.RuneError && size == 1 {
			return nil, errors.New("invalid UTF-8 input")
		}
		i += size
		if inQuotes {
			switch ch {
			case '"':
				inQuotes = false
				quoted = true
			case '\\':
				if i >= len(line) {
					return nil, errors.New("invalid escape sequence")
				}
				next, nextSize := utf8.DecodeRuneInString(line[i:])
				if next != '"' && next != '\\' {
					return nil, errors.New("invalid escape sequence")
				}
				token.WriteRune(next)
				i += nextSize
			default:
				token.WriteRune(ch)
			}
			continue
		}
		if ch == '"' {
			inQuotes = true
			quoted = true
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			if token.Len() > 0 || quoted {
				tokens = append(tokens, token.String())
				token.Reset()
				quoted = false
			}
			continue
		}
		if strings.ContainsRune("|;&$`", ch) {
			return nil, errors.New("unsupported shell syntax")
		}
		token.WriteRune(ch)
	}
	if inQuotes {
		return nil, errors.New("unterminated quote")
	}
	if token.Len() > 0 || quoted {
		tokens = append(tokens, token.String())
	}
	return tokens, nil
}

func ParseCLICommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("command is required")
	}
	if args[0] == "--export-locale-template" {
		if len(args) != 2 || args[1] == "" || strings.HasPrefix(args[1], "-") {
			return nil, errors.New("--export-locale-template requires one destination")
		}
		return ExportLocaleTemplateCommand{Destination: args[1]}, nil
	}
	switch args[0] {
	case "help":
		return exactCommand(args, HelpCommand{}, "help")
	case "version":
		return exactCommand(args, VersionCommand{}, "version")
	case "quit":
		return exactCommand(args, QuitCommand{}, "quit")
	case "language":
		return parseLanguage(args)
	case "discover":
		return parseDiscover(args)
	case "activate":
		return parseActivate(args)
	case "network":
		return parseNetwork(args)
	default:
		return nil, errors.New("unsupported command")
	}
}

func ParseConsoleCommand(line string) (Command, error) {
	args := strings.Fields(line)
	if len(args) == 0 {
		return nil, errors.New("command is required")
	}
	if strings.ContainsAny(line, "\"'\\|;&$`") {
		return nil, errors.New("console command contains unsupported syntax")
	}
	switch args[0] {
	case "discover":
		if len(args) == 1 {
			return DiscoverCommand{}, nil
		}
		if len(args) != 2 {
			return nil, errors.New("discover accepts at most one interface index")
		}
		index, err := parseNonNegativeInt(args[1], "interface index")
		if err != nil {
			return nil, err
		}
		return DiscoverCommand{InterfaceIndex: &index}, nil
	case "activate":
		return parseConsoleTarget(args, true)
	case "network":
		return parseConsoleTarget(args, false)
	case "language":
		return parseLanguage(args)
	case "help":
		return exactCommand(args, HelpCommand{}, "help")
	case "version":
		return exactCommand(args, VersionCommand{}, "version")
	case "quit":
		return exactCommand(args, QuitCommand{}, "quit")
	case "--export-locale-template":
		if len(args) != 2 || args[1] == "" {
			return nil, errors.New("template export requires one destination")
		}
		return ExportLocaleTemplateCommand{Destination: args[1]}, nil
	default:
		return nil, errors.New("unsupported command")
	}
}

func exactCommand(args []string, command Command, name string) (Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s does not accept arguments", name)
	}
	return command, nil
}

func parseLanguage(args []string) (Command, error) {
	if len(args) != 2 {
		return nil, errors.New("language requires list or a locale code")
	}
	if args[1] == "list" {
		return LanguageCommand{Action: LanguageList}, nil
	}
	if strings.TrimSpace(args[1]) == "" || strings.ContainsAny(args[1], "/\\") {
		return nil, errors.New("invalid locale code")
	}
	return LanguageCommand{Action: LanguageSwitch, Code: args[1]}, nil
}

func parseDiscover(args []string) (Command, error) {
	if len(args) < 2 {
		return nil, errors.New("discover requires --interface and --timeout")
	}
	var index *int
	var timeout time.Duration
	repeat := 1
	seenInterface, seenTimeout, seenRepeat := false, false, false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--interface":
			if seenInterface || i+1 >= len(args) {
				return nil, errors.New("invalid --interface option")
			}
			value, err := parseNonNegativeInt(args[i+1], "interface index")
			if err != nil {
				return nil, err
			}
			index, seenInterface = &value, true
			i++
		case "--timeout":
			if seenTimeout || i+1 >= len(args) {
				return nil, errors.New("invalid --timeout option")
			}
			value, err := time.ParseDuration(args[i+1])
			if err != nil || value <= 0 {
				return nil, errors.New("timeout must be positive")
			}
			timeout, seenTimeout = value, true
			i++
		case "--repeat":
			if seenRepeat || i+1 >= len(args) {
				return nil, errors.New("invalid --repeat option")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value < 1 || value > 100 {
				return nil, errors.New("repeat must be between 1 and 100")
			}
			repeat, seenRepeat = value, true
			i++
		default:
			return nil, errors.New("unknown discover option")
		}
	}
	if !seenInterface || !seenTimeout {
		return nil, errors.New("discover requires --interface and --timeout")
	}
	return DiscoverCommand{
		InterfaceIndex: index,
		Policy:         DiscoveryPolicy{CycleTimeout: timeout, RepeatCount: repeat, MaxSADPDatagramSize: 1500},
	}, nil
}

func parseActivate(args []string) (Command, error) {
	targets, password, email, confirm, next, err := parseMutationOptions(args, true)
	if err != nil {
		return nil, err
	}
	if next != len(args) || password != PasswordFromStdin {
		return nil, errors.New("activate requires --password-stdin")
	}
	return ActivateCommand{Targets: targets, PasswordInput: password, Email: email, Confirm: confirm}, nil
}

func parseNetwork(args []string) (Command, error) {
	targets, password, _, confirm, next, err := parseMutationOptions(args, false)
	if err != nil {
		return nil, err
	}
	if next >= len(args) || password != PasswordFromStdin {
		return nil, errors.New("network requires --password-stdin")
	}
	config := logic.NetworkConfig{}
	seenDHCP, seenAddress, seenGateway := false, false, false
	for i := next; i < len(args); i++ {
		switch args[i] {
		case "--dhcp":
			if seenDHCP || seenAddress {
				return nil, errors.New("network address mode is contradictory")
			}
			seenDHCP = true
		case "--address":
			if seenAddress || seenDHCP || i+1 >= len(args) {
				return nil, errors.New("invalid --address option")
			}
			address, err := netip.ParsePrefix(args[i+1])
			if err != nil || !address.Addr().Is4() {
				return nil, errors.New("address must be an IPv4 prefix")
			}
			config.Address, seenAddress = address, true
			i++
		case "--gateway":
			if seenGateway || i+1 >= len(args) {
				return nil, errors.New("invalid --gateway option")
			}
			gateway, err := netip.ParseAddr(args[i+1])
			if err != nil || !gateway.Is4() {
				return nil, errors.New("gateway must be an IPv4 address")
			}
			config.Gateway, seenGateway = gateway, true
			i++
		default:
			return nil, errors.New("unknown network option")
		}
	}
	if seenDHCP == seenAddress || (seenDHCP && seenGateway) || (seenAddress != seenGateway) {
		return nil, errors.New("network requires exactly one valid address mode")
	}
	config.UseDHCP = seenDHCP
	return NetworkCommand{Targets: targets, PasswordInput: password, Config: config, Confirm: confirm}, nil
}

func parseMutationOptions(args []string, allowEmail bool) (TargetSelector, PasswordSource, string, bool, int, error) {
	var targets []net.HardwareAddr
	var password PasswordSource
	var email string
	var confirm bool
	seenEmail, seenPassword := false, false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--device":
			if i+1 >= len(args) {
				return TargetSelector{}, 0, "", false, i, errors.New("--device requires a MAC")
			}
			mac, err := parser.NormalizeMAC(args[i+1])
			if err != nil {
				return TargetSelector{}, 0, "", false, i, errors.New("invalid device MAC")
			}
			for _, existing := range targets {
				if existing.String() == mac.String() {
					return TargetSelector{}, 0, "", false, i, errors.New("duplicate device MAC")
				}
			}
			targets = append(targets, mac)
			i++
		case "--password-stdin":
			if seenPassword {
				return TargetSelector{}, 0, "", false, i, errors.New("duplicate password source")
			}
			password, seenPassword = PasswordFromStdin, true
		case "--password":
			return TargetSelector{}, 0, "", false, i, errors.New("password arguments are not accepted")
		case "--email":
			if !allowEmail || seenEmail || i+1 >= len(args) {
				return TargetSelector{}, 0, "", false, i, errors.New("invalid --email option")
			}
			email, seenEmail = args[i+1], true
			i++
		case "--yes":
			if confirm {
				return TargetSelector{}, 0, "", false, i, errors.New("duplicate confirmation")
			}
			confirm = true
		default:
			if !allowEmail && (args[i] == "--dhcp" || args[i] == "--address" || args[i] == "--gateway") {
				return TargetSelector{MACs: targets}, password, email, confirm, i, nil
			}
			return TargetSelector{}, 0, "", false, i, errors.New("unknown mutation option")
		}
	}
	if len(targets) == 0 {
		return TargetSelector{}, 0, "", false, len(args), errors.New("at least one device is required")
	}
	if len(targets) > 1 && !confirm {
		return TargetSelector{}, 0, "", false, len(args), errors.New("multiple devices require --yes")
	}
	return TargetSelector{MACs: targets}, password, email, confirm, len(args), nil
}

func parseConsoleTarget(args []string, activate bool) (Command, error) {
	if len(args) == 1 {
		if activate {
			return ActivateCommand{Targets: TargetSelector{UseFocusedDevice: true}}, nil
		}
		return NetworkCommand{Targets: TargetSelector{UseFocusedDevice: true}}, nil
	}
	if len(args) != 2 {
		return nil, errors.New("command accepts at most one device MAC")
	}
	mac, err := parser.NormalizeMAC(args[1])
	if err != nil {
		return nil, errors.New("invalid device MAC")
	}
	targets := TargetSelector{MACs: []net.HardwareAddr{mac}}
	if activate {
		return ActivateCommand{Targets: targets}, nil
	}
	return NetworkCommand{Targets: targets}, nil
}

func parseNonNegativeInt(value, name string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return number, nil
}
