# Procedure Specification: P4.5 - DispatchCommand
## Module: M4 - Command-Line Interface

## 1. Purpose

Define the shared typed command catalog, dispatch boundary, and result envelope used by the TUI command console, CLI REPL, and one-shot CLI. This procedure routes validated application requests; it does not own terminal parsing or protocol implementation.

## 2. Go Contract

```go
type Command interface {
    isCommand()
}

type DiscoverCommand struct {
    InterfaceIndex *int
    Policy         DiscoveryPolicy
}

type DiscoveryPolicy struct {
    ProbeInterval time.Duration
    CycleTimeout  time.Duration
    RepeatCount   int
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
    Config        NetworkConfig
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

type TargetSelector struct {
    MACs              []net.HardwareAddr
    UseTUISelection   bool
    UseFocusedDevice  bool
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

func DispatchCommand(
    ctx context.Context,
    command Command,
    dependencies Dependencies,
) (CommandResult, error)
```

`NetworkConfig` is the typed M3 configuration contract, not a separate M4 model. `Dependencies` supplies the application services used by dispatch, including a TUI-owned discovery-session manager. Command variants are closed to the M4 package; both parsers construct these variants. `DispatchCommand` must be the only common command-to-workflow routing boundary.

- `PasswordFromPrompt` is used by the TUI's masked prompt; `PasswordFromStdin` is used by CLI `--password-stdin`. Neither source is read until all command, target, confirmation, and protocol-gate checks succeed.
- `LanguageList` lists available locales; `LanguageSwitch` uses the exact `Code` value. A switch does not persist settings.
- `TargetSelector` must select exactly one strategy: one or more explicit `MACs`, `UseTUISelection`, or `UseFocusedDevice`. Reject empty or conflicting selectors; resolution confirms the selected devices exist before any secret input.

- A continuous `DiscoveryPolicy` has `ProbeInterval > 0`, `CycleTimeout == 0`, and `RepeatCount == 0`.
- A bounded `DiscoveryPolicy` has `ProbeInterval == 0`, `CycleTimeout > 0`, and `RepeatCount` in `1..100`.
- Reject policies that mix or omit these modes. CLI uses bounded policy; TUI uses continuous policy with a 60-second probe interval.

## 3. Dispatch and Target Resolution

- Reject nil context, nil command, invalid typed fields, or invalid dependency configuration before side effects.
- `DiscoverCommand` invokes M6 with the selected policy. The TUI owns at most one continuous discovery session: it starts immediately, keeps receiving, and sends a new probe every 60 seconds until canceled. TUI dispatch starts or switches this background session without blocking the UI event loop and returns an acknowledgement; discovery events are marshaled to the UI event loop before updating its collector. Re-selecting the active interface does not start a duplicate session; selecting a different interface cancels and joins the old session before starting the new one. CLI starts the first bounded cycle immediately; every cycle sends one probe and keeps receiving for the full timeout, then starts the next cycle immediately. A received device does not shorten a cycle.
- An absent TUI interface index is resolved from the explicitly selected interface before dispatch; absence is an actionable error. CLI requires the index during parsing.
- For TUI mutation commands, an explicit MAC targets that device. Otherwise resolve all currently selected devices; if selection is empty, resolve the focused device. Reject an empty/unavailable target set before collecting a password or dispatching a mutation.
- For CLI mutation commands, resolve every explicitly supplied MAC against the current device inventory. Reject unknown or ambiguous targets; never silently skip a target. Preserve input order for per-device outcomes.
- Require explicit confirmation for two or more mutation targets. The TUI presents a confirmation showing the exact target set; CLI parsing requires `--yes` for multiple targets and does not prompt in one-shot mode.
- Preserve one `DeviceOutcome` per requested target, with status and a safe error summary. Do not collapse a partial batch result into overall success. `CommandResult.OK` is true only if the command succeeds for all requested targets.
- `help`, `version`, `quit`, `language`, and template export route through their corresponding M4/M5 service. `quit` is an REPL control result; in one-shot it succeeds without starting another command.
- Until the device-management protocol gate is lifted, `ActivateCommand` and `NetworkCommand` return `CommandFailure{Code:"unsupported"}` before resolving secrets, reading password input, prompting, or opening sockets.
- Use stable failure codes including `invalid_argument`, `unsupported`, `operation_failed`, and `canceled`. Sanitize `Message`; it must not expose secrets, raw payloads, or sensitive filesystem contents.

## 4. Discovery Aggregation

- M6 emits validated advertisement events for each received packet; it does not infer UUID correlation or suppress raw protocol events.
- The shared M4 discovery collector builds the user-facing inventory using `(normalized MAC, sender network address)` as the unique key. A repeated advertisement updates that device's latest validated fields and last-seen time without creating a duplicate row.
- CLI `discover` returns a final snapshot sorted by normalized MAC, then sender address. No devices is a successful empty result. TUI updates its list as events arrive and retains selection for identities that remain present.
- The aggregation key is presentation/application policy, not a protocol-level identity or response-correlation guarantee.

## 5. Output and Process Policy

- Render `CommandResult` as text or JSON according to the process-wide CLI `--format`. JSON has one object per completed command; the REPL emits one object per line to stdout. The REPL prompt/input UI and diagnostic logs are written to stderr in every format.
- Command-level validation and workflow failures are represented in `CommandResult` and, in JSON mode, remain machine-readable on stdout. Diagnostics must not contain passwords, raw payloads, or secret fields. Do not emit a second JSON object for the same command.
- In REPL mode, a parse/tokenization error is also emitted as one result with `OK=false`, `Error.Code="invalid_argument"`, and omitted `Command`; the REPL then continues. One-shot parse errors use exit code 2 and the same JSON error envelope when JSON output was requested.
- In REPL mode, a completed failure result does not terminate the process. In one-shot mode, map success to exit code 0, invalid invocation to 2, operational/unsupported failure to 1, and interruption to 130.
- Never log command history containing secrets, raw XML, full device payloads, or values such as `Salt`.

## 6. Errors and Tests

Return an error only when dispatch cannot produce a valid command result (for example, invalid internal command/dependency contract); expected command/workflow failures are represented in `CommandResult` with `OK=false` and a safe `CommandFailure`. Preserve underlying errors for internal handling without exposing sensitive values.

Test every command variant, malformed typed requests, target resolution and ambiguity, duplicate normalized targets, selection-vs-focus precedence, multi-target confirmation, per-device partial outcomes, protocol-gate short-circuit before password input/network access, discovery aggregation and deterministic ordering, empty discovery, M5 routing, JSON result shape, redaction, and exit-status mapping.

---

*Level 2 procedure specification.*
