package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logging"
	"github.com/CDSRaBot/sadp-handler.GO/internal/network"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiCommandResultMsg struct {
	result CommandResult
	err    error
}

type tuiDiscoverySessionMsg struct {
	session    DiscoverySession
	err        error
	generation uint64
}

type tuiDiscoveryEventMsg struct {
	device     DiscoveredDevice
	err        error
	done       bool
	generation uint64
}

type tuiInterfacesMsg struct {
	interfaces []adapter.Interface
	err        error
}

type TUIModel struct {
	state               *TUIState
	dependencies        Dependencies
	ctx                 context.Context
	devices             list.Model
	filter              textinput.Model
	console             textarea.Model
	details             viewport.Model
	session             DiscoverySession
	discoveryCancel     context.CancelFunc
	discoveryDone       <-chan struct{}
	discoveryGeneration uint64
	discovered          []parser.Device
	interfaces          []adapter.Interface
	interfacePicker     bool
	interfaceCursor     int
	pending             Command
	status              string
	width               int
	height              int
}

func RunTUI(ctx context.Context, dependencies Dependencies) error {
	if ctx == nil {
		return fmt.Errorf("TUI context is nil")
	}
	model := NewTUIModel(ctx, NewTUIState(), dependencies)
	_, err := tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen()).Run()
	return err
}

func NewTUIModel(ctx context.Context, state *TUIState, dependencies Dependencies) TUIModel {
	if ctx == nil {
		ctx = context.Background()
	}
	if state == nil {
		state = NewTUIState()
	}
	input := textarea.New()
	input.Placeholder = "Enter command..."
	input.Prompt = "> "
	input.CharLimit = 4096
	input.SetHeight(1)
	input.ShowLineNumbers = false
	filter := textinput.New()
	filter.Prompt = "Filter: "
	filter.CharLimit = 256
	filter.Placeholder = "type to filter devices"

	items := make([]list.Item, 0)
	devices := list.New(items, deviceDelegate{}, 0, 0)
	devices.Title = "Devices"
	devices.SetShowStatusBar(false)
	devices.SetShowPagination(false)
	devices.SetShowTitle(false)
	devices.SetShowHelp(false)
	devices.SetFilteringEnabled(false)

	return TUIModel{
		state:        state,
		dependencies: dependencies,
		ctx:          ctx,
		devices:      devices,
		filter:       filter,
		console:      input,
		details:      viewport.New(0, 0),
		status:       "i: interface  Tab: zone  Arrows: navigate  Space: select  Enter: open  Ctrl+C: exit",
	}
}

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.loadInterfaces())
}

func (m TUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resize()
	case tuiCommandResultMsg:
		if message.result.Error != nil {
			m.status = message.result.Error.Message
		} else if message.err != nil {
			m.status = fmt.Sprintf("command failed: %v", message.err)
		} else if message.result.ISAPIStatus != nil {
			m.status = formatISAPIStatus(*message.result.ISAPIStatus)
		} else {
			m.status = message.result.Message
		}
		m.details.SetContent(m.status)
		if message.result.Exit {
			m.stopDiscovery()
			return m, tea.Quit
		}
	case tuiDiscoverySessionMsg:
		if message.generation != m.discoveryGeneration {
			return m, nil
		}
		if message.err != nil {
			m.status = "discovery failed to start: " + sanitizeTUIDisplay(message.err.Error())
			m.logEvent(slog.LevelError, "ui.discovery_start_failed")
			return m, nil
		}
		if message.session == nil || message.session.Events() == nil || message.session.Errors() == nil {
			m.status = "discovery session is unavailable"
			m.logEvent(slog.LevelError, "ui.discovery_start_failed")
			return m, nil
		}
		m.session = message.session
		m.status = "discovery running"
		m.logEvent(slog.LevelInfo, "ui.discovery_started")
		return m, m.waitForDiscoveryEvent()
	case tuiInterfacesMsg:
		if message.err != nil {
			m.status = fmt.Sprintf("interface enumeration failed: %v", message.err)
			return m, nil
		}
		m.SetAvailableInterfaces(message.interfaces)
		if m.dependencies.InterfaceIndex == nil {
			m.status = "press i to choose an interface"
			return m, nil
		}
		if err := m.SelectInterface(*m.dependencies.InterfaceIndex); err != nil {
			m.status = "configured interface is unavailable"
			return m, nil
		}
		return m, m.startDiscovery()
	case tuiDiscoveryEventMsg:
		if message.generation != m.discoveryGeneration {
			return m, nil
		}
		if message.err != nil {
			m.status = "discovery error: " + sanitizeTUIDisplay(message.err.Error())
			m.logEvent(slog.LevelWarn, "ui.discovery_error")
			return m, m.waitForDiscoveryEvent()
		}
		if message.done {
			m.stopDiscovery()
			m.status = "discovery stopped"
			m.logEvent(slog.LevelInfo, "ui.discovery_stopped")
			return m, nil
		}
		m.applyDiscoveryEvent(message.device)
		return m, m.waitForDiscoveryEvent()
	case tea.KeyMsg:
		if message.String() == "ctrl+c" {
			m.stopDiscovery()
			return m, tea.Quit
		}
		if m.interfacePicker {
			return m.handleInterfacePicker(message)
		}
		if m.pending != nil {
			switch message.String() {
			case "y", "Y":
				command := confirmedCommand(m.pending)
				m.pending = nil
				return m, m.runCommand(command)
			case "n", "N", "esc":
				m.pending = nil
				m.status = "operation canceled"
				return m, nil
			default:
				return m, nil
			}
		}
		switch message.String() {
		case "tab":
			m.nextZone()
			return m, nil
		case "shift+tab":
			m.previousZone()
			return m, nil
		}
		switch m.state.Zone() {
		case TUIZoneDevices:
			if m.filterActive() {
				switch message.String() {
				case "esc":
					m.filter.Blur()
					m.filter.SetValue("")
					m.state.SetFilter("")
					m.syncDeviceList()
					m.status = "filter cleared"
					return m, nil
				case "enter":
					m.filter.Blur()
					m.status = fmt.Sprintf("%d device(s) match filter", len(m.state.VisibleDevices()))
					return m, nil
				default:
					var command tea.Cmd
					m.filter, command = m.filter.Update(message)
					m.state.SetFilter(m.filter.Value())
					m.syncDeviceList()
					return m, command
				}
			}
			switch message.String() {
			case "up":
				m.state.MoveFocus(-1)
			case "down":
				m.state.MoveFocus(1)
			case " ":
				m.state.ToggleFocusedSelection()
			case "enter":
				if err := m.state.SetZone(TUIZoneDetails); err != nil {
					m.status = "cannot open device details"
				} else {
					m.updateFocus()
					m.updateDeviceDetails()
				}
			case "i":
				m.openInterfacePicker()
				return m, nil
			case "/":
				m.filter.Focus()
				m.status = "filtering devices; Enter accepts, Esc clears"
				return m, textinput.Blink
			default:
				if message.Type == tea.KeyRunes && len(message.Runes) == 1 && message.Runes[0] == '/' {
					m.filter.Focus()
					m.status = "filtering devices; Enter accepts, Esc clears"
					return m, textinput.Blink
				}
				if message.String() == "q" {
					m.stopDiscovery()
					return m, tea.Quit
				}
			}
			m.syncDeviceList()
			m.updateDeviceDetails()
		case TUIZoneDetails:
			switch message.String() {
			case "esc":
				if err := m.state.SetZone(TUIZoneDevices); err != nil {
					m.status = "cannot return to device list"
				} else {
					m.updateFocus()
				}
			default:
				var command tea.Cmd
				m.details, command = m.details.Update(message)
				return m, command
			}
		case TUIZoneConsole:
			if message.String() == "enter" {
				return m, m.submitConsole()
			}
			if message.String() == "up" {
				if value, ok := m.state.PreviousHistory(); ok {
					m.console.SetValue(value)
				}
				return m, nil
			}
			if message.String() == "down" {
				if value, ok := m.state.NextHistory(); ok {
					m.console.SetValue(value)
				}
				return m, nil
			}
			var command tea.Cmd
			m.console, command = m.console.Update(message)
			return m, command
		}
	}
	return m, nil
}

func (m TUIModel) View() string {
	top := lipgloss.NewStyle().Bold(true).Render("SADP Handler  " + m.interfaceStatus())
	if m.interfacePicker {
		return lipgloss.JoinVertical(lipgloss.Left, top, m.interfacePickerView(), m.status)
	}
	filter := m.filter.View()
	deviceTitle, detailTitle, consoleTitle := "Devices", "Details", "Console"
	switch m.state.Zone() {
	case TUIZoneDevices:
		deviceTitle += " [active]"
	case TUIZoneDetails:
		detailTitle += " [active]"
	case TUIZoneConsole:
		consoleTitle += " [active]"
	}
	leftWidth, rightWidth, bodyHeight := m.layoutDimensions()
	deviceView := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(leftWidth - 2).
		Height(bodyHeight - 2).Render(lipgloss.JoinVertical(lipgloss.Left, deviceTitle, m.devices.View()))
	detailView := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(rightWidth - 2).
		Height(bodyHeight - 2).Render(lipgloss.JoinVertical(lipgloss.Left, detailTitle, m.details.View()))
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		deviceView, detailView,
	)
	console := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(leftWidth + rightWidth - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, consoleTitle, m.console.View()))
	return lipgloss.JoinVertical(lipgloss.Left, top, filter, body, console, m.status)
}

func (m TUIModel) filterActive() bool {
	return m.filter.Focused()
}

func (m *TUIModel) SetDevices(devices []parser.Device) {
	if m == nil {
		return
	}
	m.state.SetDevices(devices)
	m.syncDeviceList()
}

func (m *TUIModel) startDiscovery() tea.Cmd {
	if m.dependencies.DiscoverySession == nil {
		m.status = "discovery service is unavailable"
		return nil
	}
	if m.dependencies.InterfaceIndex == nil {
		m.status = "select an interface before discovery"
		return nil
	}
	previousDone := m.discoveryDone
	m.stopDiscovery()
	m.discoveryGeneration++
	generation := m.discoveryGeneration
	ctx, cancel := context.WithCancel(m.ctx)
	m.discoveryCancel = cancel
	done := make(chan struct{})
	m.discoveryDone = done
	service := m.dependencies.DiscoverySession
	selected := *m.dependencies.InterfaceIndex
	index := &selected
	policy := m.dependencies.DiscoveryPolicy
	if policy.ProbeInterval <= 0 {
		policy.ProbeInterval = time.Minute
	}
	policy.CycleTimeout = 0
	policy.RepeatCount = 0
	if policy.MaxSADPDatagramSize == 0 {
		policy.MaxSADPDatagramSize = network.DefaultMaxSADPDatagramSize
	}
	m.status = "starting discovery"
	m.logEvent(slog.LevelInfo, "ui.discovery_start_requested")
	return func() tea.Msg {
		if previousDone != nil {
			select {
			case <-previousDone:
			case <-ctx.Done():
				close(done)
				return tuiDiscoverySessionMsg{err: ctx.Err(), generation: generation}
			}
		}
		session, err := service.Start(ctx, index, policy)
		if err != nil {
			close(done)
			return tuiDiscoverySessionMsg{err: err, generation: generation}
		}
		if session == nil || session.Events() == nil || session.Errors() == nil {
			close(done)
			return tuiDiscoverySessionMsg{err: errors.New("discovery session channels are incomplete"), generation: generation}
		}
		forwarded := newTUIForwardedSession(ctx, session, done)
		return tuiDiscoverySessionMsg{session: forwarded, generation: generation}
	}
}

func (m *TUIModel) stopDiscovery() {
	if m.discoveryCancel != nil {
		m.discoveryCancel()
		m.discoveryCancel = nil
	}
	m.session = nil
}

func (m *TUIModel) logEvent(level slog.Level, name string) {
	if m.dependencies.Logger == nil {
		return
	}
	if err := logging.Event(m.dependencies.Logger, level, name); err != nil {
		m.status += " | log write failed"
	}
}

func (m TUIModel) loadInterfaces() tea.Cmd {
	if m.dependencies.InterfaceLister == nil {
		return nil
	}
	lister := m.dependencies.InterfaceLister
	return func() tea.Msg {
		interfaces, err := lister.ListEligibleInterfaces()
		return tuiInterfacesMsg{interfaces: interfaces, err: err}
	}
}

func (m *TUIModel) openInterfacePicker() {
	if len(m.interfaces) == 0 {
		m.status = "no eligible interface available"
		return
	}
	m.interfaceCursor = 0
	if selected, ok := m.SelectedInterface(); ok {
		for index, iface := range m.interfaces {
			if iface.Index == selected.Index {
				m.interfaceCursor = index
				break
			}
		}
	}
	m.interfacePicker = true
	m.status = "choose interface: arrows, Enter to select, Esc to cancel"
}

func (m TUIModel) interfaceStatus() string {
	if selected, ok := m.SelectedInterface(); ok {
		return fmt.Sprintf("interface %d (%s)", selected.Index, selected.Name)
	}
	return "interface: none (press i to select)"
}

func (m TUIModel) waitForDiscoveryEvent() tea.Cmd {
	session := m.session
	generation := m.discoveryGeneration
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		events, errs := session.Events(), session.Errors()
		for events != nil || errs != nil {
			select {
			case device, ok := <-events:
				if !ok {
					events = nil
					continue
				}
				return tuiDiscoveryEventMsg{device: device, generation: generation}
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				if err != nil {
					return tuiDiscoveryEventMsg{err: err, generation: generation}
				}
				return tuiDiscoveryEventMsg{err: errors.New("discovery session returned an empty error"), generation: generation}
			}
		}
		return tuiDiscoveryEventMsg{done: true, generation: generation}
	}
}

func (m *TUIModel) applyDiscoveryEvent(device DiscoveredDevice) {
	converted, ok := toParserDevice(device)
	if !ok {
		m.status = "received invalid discovery event"
		return
	}
	identity := converted.MAC.String() + "|" + converted.Sender.String()
	for index, existing := range m.discovered {
		if existing.MAC.String()+"|"+existing.Sender.String() == identity {
			m.discovered[index] = converted
			m.state.SetDevices(m.discovered)
			m.syncDeviceList()
			m.updateDeviceDetails()
			return
		}
	}
	m.discovered = append(m.discovered, converted)
	m.state.SetDevices(m.discovered)
	m.syncDeviceList()
	m.updateDeviceDetails()
	m.status = fmt.Sprintf("%d device(s) discovered", len(m.discovered))
	m.logEvent(slog.LevelInfo, "ui.device_discovered")
}

func toParserDevice(device DiscoveredDevice) (parser.Device, bool) {
	mac, err := net.ParseMAC(device.MAC)
	if err != nil || len(mac) != 6 {
		return parser.Device{}, false
	}
	ipv4, err := netip.ParseAddr(device.IPv4)
	if err != nil || !ipv4.Is4() {
		return parser.Device{}, false
	}
	sender := parseSenderIP(device.Sender)
	if sender == nil {
		return parser.Device{}, false
	}
	return parser.Device{
		MAC:             mac,
		IPv4:            ipv4,
		Activated:       device.Activated,
		ActivationKnown: device.ActivationKnown,
		Model:           device.Model,
		SerialNumber:    device.SerialNumber,
		SoftwareVersion: device.SoftwareVersion,
		Sender:          &net.IPAddr{IP: sender},
	}, true
}

func parseSenderIP(value string) net.IP {
	if ip := net.ParseIP(value); ip != nil {
		return ip
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func (m *TUIModel) syncDeviceList() {
	items := make([]list.Item, 0)
	for _, device := range m.state.VisibleDevices() {
		_, selected := m.state.selected[deviceIdentity(device)]
		items = append(items, deviceItem{device: device, selected: selected})
	}
	m.devices.SetItems(items)
	if len(items) != 0 {
		m.devices.Select(m.state.focused)
	}
}

func (m *TUIModel) resize() {
	if m.width < 1 || m.height < 1 {
		return
	}
	leftWidth, rightWidth, bodyHeight := m.layoutDimensions()
	m.devices.SetSize(leftWidth-2, bodyHeight-3)
	m.details.Width = rightWidth - 2
	m.details.Height = bodyHeight - 3
	m.console.SetWidth(leftWidth + rightWidth - 4)
}

func (m TUIModel) layoutDimensions() (int, int, int) {
	width, height := m.width, m.height
	if width < 20 {
		width = 20
	}
	if height < 12 {
		height = 12
	}
	return width / 2, width - width/2, height - 8
}

func (m *TUIModel) updateDeviceDetails() {
	device, ok := m.state.FocusedDevice()
	if !ok {
		m.details.SetContent("No device selected")
		return
	}
	state := discoveryActivationState(device.ActivationKnown, device.Activated)
	details := fmt.Sprintf("Model: %s\nSerial: %s\nIP: %s\nMAC: %s\nActivation: %s\nFirmware: %s\nSADP identity unverified",
		sanitizeTUIDisplay(device.Model), sanitizeTUIDisplay(device.SerialNumber), device.IPv4,
		device.MAC, state, sanitizeTUIDisplay(device.SoftwareVersion))
	m.details.SetContent(details)
}

func (m *TUIModel) nextZone() {
	zone := m.state.Zone() + 1
	if zone > TUIZoneConsole {
		zone = TUIZoneDevices
	}
	if err := m.state.SetZone(zone); err != nil {
		m.status = fmt.Sprintf("change zone failed: %v", err)
		return
	}
	m.updateFocus()
}

func (m *TUIModel) previousZone() {
	zone := m.state.Zone() - 1
	if zone < TUIZoneDevices {
		zone = TUIZoneConsole
	}
	if err := m.state.SetZone(zone); err != nil {
		m.status = fmt.Sprintf("change zone failed: %v", err)
		return
	}
	m.updateFocus()
}

func (m *TUIModel) updateFocus() {
	m.console.Blur()
	m.filter.Blur()
	if m.state.Zone() == TUIZoneConsole {
		m.console.Focus()
	}
}

func (m *TUIModel) submitConsole() tea.Cmd {
	line := strings.TrimSpace(m.console.Value())
	m.console.Reset()
	command, err := ParseConsoleCommand(line)
	if err != nil {
		m.status = "invalid command"
		return nil
	}
	if err := m.state.AddHistory(line); err != nil {
		m.status = "command was not stored in history"
		return nil
	}
	if discover, ok := command.(DiscoverCommand); ok {
		m.logEvent(slog.LevelInfo, "ui.command_discover")
		if discover.InterfaceIndex != nil {
			if err := m.SelectInterface(*discover.InterfaceIndex); err != nil {
				m.status = "requested interface is unavailable"
				return nil
			}
		} else if m.dependencies.InterfaceIndex == nil {
			m.openInterfacePicker()
			return nil
		}
		return m.startDiscovery()
	}
	if _, ok := command.(ActivateCommand); ok &&
		(m.dependencies.Activation == nil || sadpActivationDisabled(m.dependencies.Activation)) {
		return m.runCommand(command)
	}
	resolved, err := ResolveTUICommand(m.state, command)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	if message := confirmationMessage(resolved); message != "" {
		m.pending = resolved
		m.status = message
		return nil
	}
	return m.runCommand(resolved)
}

func (m TUIModel) runCommand(command Command) tea.Cmd {
	ctx := m.ctx
	dependencies := m.dependencies
	return func() tea.Msg {
		result, err := DispatchCommand(ctx, command, dependencies)
		return tuiCommandResultMsg{result: result, err: err}
	}
}

func confirmedCommand(command Command) Command {
	switch value := command.(type) {
	case ActivateCommand:
		value.Confirm = true
		return value
	case NetworkCommand:
		value.Confirm = true
		return value
	default:
		return command
	}
}

type deviceItem struct {
	device   parser.Device
	selected bool
}

func (i deviceItem) FilterValue() string {
	return i.device.MAC.String() + " " + i.device.Model + " " + i.device.SerialNumber
}

type deviceDelegate struct{}

func (deviceDelegate) Height() int  { return 1 }
func (deviceDelegate) Spacing() int { return 0 }
func (deviceDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}
func (deviceDelegate) Render(writer io.Writer, model list.Model, index int, item list.Item) {
	value, ok := item.(deviceItem)
	if !ok {
		return
	}
	state := discoveryActivationState(value.device.ActivationKnown, value.device.Activated)
	cursor, selection := " ", "[ ]"
	if index == model.Index() {
		cursor = ">"
	}
	if value.selected {
		selection = "[x]"
	}
	row := fmt.Sprintf("%s%s %s  %-8s  %s  %s", cursor, selection, value.device.MAC, state,
		sanitizeTUIDisplay(value.device.Model), sanitizeTUIDisplay(value.device.SerialNumber))
	_, _ = fmt.Fprint(writer, lipgloss.NewStyle().MaxWidth(model.Width()).Render(row))
}
