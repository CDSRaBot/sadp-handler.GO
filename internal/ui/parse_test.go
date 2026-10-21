package ui_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

func TestTokenizeCLIInputSupportsQuotingAndOnlyDocumentedEscapes(t *testing.T) {
	testCases := []struct {
		name string
		line string
		want []string
	}{
		{name: "unicode whitespace", line: "language\t ru\n", want: []string{"language", "ru"}},
		{name: "quoted whitespace", line: `discover --interface "Wi Fi"`, want: []string{"discover", "--interface", "Wi Fi"}},
		{name: "escaped quote", line: `language "en\"-US"`, want: []string{"language", `en"-US`}},
		{name: "escaped backslash", line: `language "C:\\locale"`, want: []string{"language", `C:\locale`}},
		{name: "backslash outside quotes is literal", line: `language en\US`, want: []string{"language", `en\US`}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ui.TokenizeCLIInput(testCase.line)
			if err != nil {
				t.Fatalf("TokenizeCLIInput returned error: %v", err)
			}
			if len(got) != len(testCase.want) {
				t.Fatalf("tokens = %#v, want %#v", got, testCase.want)
			}
			for i := range got {
				if got[i] != testCase.want[i] {
					t.Fatalf("tokens = %#v, want %#v", got, testCase.want)
				}
			}
		})
	}
}

func TestTokenizeCLIInputRejectsMalformedQuotesAndShellSyntax(t *testing.T) {
	for _, line := range []string{`language "unterminated`, `language "bad\q"`, `discover | whoami`, `echo $(secret)`} {
		t.Run(line, func(t *testing.T) {
			tokens, err := ui.TokenizeCLIInput(line)
			if err == nil || tokens != nil {
				t.Fatalf("TokenizeCLIInput(%q) = %#v, %v; want safe error and nil", line, tokens, err)
			}
		})
	}
}

func TestParseCLICommandBuildsSharedCommandVariants(t *testing.T) {
	testCases := []struct {
		name string
		args []string
		want any
	}{
		{name: "help", args: []string{"help"}, want: ui.HelpCommand{}},
		{name: "version", args: []string{"version"}, want: ui.VersionCommand{}},
		{name: "quit", args: []string{"quit"}, want: ui.QuitCommand{}},
		{name: "language list", args: []string{"language", "list"}, want: ui.LanguageCommand{Action: ui.LanguageList}},
		{name: "language switch", args: []string{"language", "ru"}, want: ui.LanguageCommand{Action: ui.LanguageSwitch, Code: "ru"}},
		{name: "discover", args: []string{"discover", "--interface", "2", "--timeout", "3s"}, want: ui.DiscoverCommand{}},
		{name: "export locale", args: []string{"--export-locale-template", "locale.json"}, want: ui.ExportLocaleTemplateCommand{Destination: "locale.json"}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			command, err := ui.ParseCLICommand(testCase.args)
			if err != nil {
				t.Fatalf("ParseCLICommand returned error: %v", err)
			}
			if command == nil {
				t.Fatalf("ParseCLICommand returned nil command")
			}
			switch want := testCase.want.(type) {
			case ui.HelpCommand:
				if _, ok := command.(ui.HelpCommand); !ok {
					t.Fatalf("command = %T, want HelpCommand", command)
				}
			case ui.VersionCommand:
				if _, ok := command.(ui.VersionCommand); !ok {
					t.Fatalf("command = %T, want VersionCommand", command)
				}
			case ui.QuitCommand:
				if _, ok := command.(ui.QuitCommand); !ok {
					t.Fatalf("command = %T, want QuitCommand", command)
				}
			case ui.LanguageCommand:
				if !reflect.DeepEqual(command, want) {
					t.Fatalf("command = %#v, want %#v", command, want)
				}
			case ui.ExportLocaleTemplateCommand:
				if !reflect.DeepEqual(command, want) {
					t.Fatalf("command = %#v, want %#v", command, want)
				}
			}
		})
	}
}

func TestParseCLICommandRejectsInvalidFormsWithoutCommand(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"unknown"},
		{"help", "extra"},
		{"discover", "--interface", "-1", "--timeout", "1s"},
		{"discover", "--interface", "1"},
		{"discover", "--interface", "1", "--timeout", "0s"},
		{"activate", "--device", "00:11:22:33:44:55"},
		{"activate", "--device", "00:11:22:33:44:55", "--password-stdin", "--device", "00:11:22:33:44:55"},
		{"network", "--device", "00:11:22:33:44:55", "--password-stdin", "--dhcp", "--address", "192.0.2.20/24"},
		{"language"},
		{"--export-locale-template", "a.json", "help"},
	} {
		t.Run(argsName(args), func(t *testing.T) {
			command, err := ui.ParseCLICommand(args)
			if err == nil || command != nil {
				t.Fatalf("ParseCLICommand(%#v) = %#v, %v; want safe error and nil", args, command, err)
			}
		})
	}
}

func TestParseConsoleCommandUsesApplicationGrammarWithoutShellExecution(t *testing.T) {
	for _, line := range []string{"help", "version", "quit", "language list", "discover 2", "activate 00:11:22:33:44:55"} {
		t.Run(line, func(t *testing.T) {
			command, err := ui.ParseConsoleCommand(line)
			if err != nil || command == nil {
				t.Fatalf("ParseConsoleCommand(%q) = %#v, %v; want command", line, command, err)
			}
		})
	}
	for _, line := range []string{"", "   ", "help extra", "language en extra", "echo hello", `quit; whoami`, `$(whoami)`} {
		t.Run("reject "+line, func(t *testing.T) {
			command, err := ui.ParseConsoleCommand(line)
			if err == nil || command != nil {
				t.Fatalf("ParseConsoleCommand(%q) = %#v, %v; want safe error and nil", line, command, err)
			}
		})
	}
}

func TestParsersDoNotPerformIOOrDiscloseSensitiveInput(t *testing.T) {
	const secret = "password-canary-123!"
	command, err := ui.ParseCLICommand([]string{"activate", "--device", "00:11:22:33:44:55", "--password", secret})
	if err == nil || command != nil {
		t.Fatalf("expected parse failure before secret handling, got command=%#v error=%v", command, err)
	}
	if errors.Is(err, contextCanceledSentinel{}) {
		t.Fatal("parser unexpectedly performed cancellable work")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("parser error disclosed secret: %v", err)
	}
}

func argsName(args []string) string {
	if len(args) == 0 {
		return "empty"
	}
	return args[0]
}

type contextCanceledSentinel struct{}

func (contextCanceledSentinel) Error() string { return "context canceled" }
