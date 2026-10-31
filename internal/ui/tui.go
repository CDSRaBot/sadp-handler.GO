package ui

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
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
	session DiscoverySession
	err     error
}

type tuiDiscoveryEventMsg struct {
	device DiscoveredDevice
	err    error
	done   bool
}

type tuiInterfacesMsg struct {
	interfaces []adapter.Interface
	err        error
}

type TUIModel struct {
	state        *TUIState
	dependencies Dependencies
	ctx          context.Context
	devices      list.Model
	filter       textinput.Model
	console      textarea.Model
	details      viewport.Model
	session      DiscoverySession
	discovered   []parser.Device
	interfaces   []adapter.Interface
	pending      Command
	status       string
	width        int
	height       int
}

func RunTUI(ctx context.Context, dependencies Dependencies) error {
	if ctx == nil {
		return fmt.Errorf("TUI context is nil")
	}
	model := NewTUIModel(ctx, NewTUIState(), dependencies)
	_, err := tea.NewProgram(model, tea.WithContext(ctx)).Run()
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
	devices.SetFilteringEnabled(false)

	return TUIModel{
		state:        state,
		dependencies: dependencies,
		ctx:          ctx,
		devices:      devices,
		filter:       filter,
		console:      input,
		details:      viewport.New(0, 0),
		status:       "Tab: switch zone  Enter: submit  Space: select  Ctrl+C: exit",
	}
}

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.loadInterfaces(), m.startDiscovery())
}

func (m TUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.resize()
	case tuiCommandResultMsg:
		if message.err != nil {
			m.status = fmt.Sprintf("command failed: %v", message.err)
		} else if message.result.Error != nil {
			m.status = message.result.Error.Message
		} else {
			m.status = message.result.Message
		}
		m.details.SetContent(m.status)
	case tuiDiscoverySessionMsg:
		if message.err != nil {
			m.status = fmt.Sprintf("discovery failed to start: %v", message.err)
			return m, nil
		}
		m.session = message.session
		return m, m.waitForDiscoveryEvent()
	case tuiInterfacesMsg:
		if message.err != nil {
			m.status = fmt.Sprintf("interface enumeration failed: %v", message.err)
			return m, nil
		}
		m.SetAvailableInterfaces(message.interfaces)
		if m.dependencies.InterfaceIndex == nil {
			m.status = "select an interface before discovery"
			return m, nil
		}
		if err := m.SelectInterface(*m.dependencies.InterfaceIndex); err != nil {
			m.status = "configured interface is unavailable"
			return m, nil
		}
		return m, m.startDiscovery()
	case tuiDiscoveryEventMsg:
		if message.err != nil {
			m.status = fmt.Sprintf("discovery failed: %v", message.err)
			return m, nil
		}
		if message.done {
			m.status = "discovery stopped"
			return m, nil
		}
		m.applyDiscoveryEvent(message.device)
		return m, m.waitForDiscoveryEvent()
	case tea.KeyMsg:
		if message.String() == "ctrl+c" {
			return m, tea.Quit
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
				m.state.SetZone(TUIZoneDetails)
			case "i":
				return m, m.selectNextInterface()
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
					return m, tea.Quit
				}
			}
			m.syncDeviceList()
		case TUIZoneDetails:
			if message.String() == "esc" {
				m.state.SetZone(TUIZoneDevices)
			}
		case TUIZoneConsole:
			if message.String() == "enter" {
				return m, m.submitConsole()
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
	filter := m.filter.View()
	deviceView := lipgloss.JoinVertical(lipgloss.Left, "Devices", m.devices.View())
	detailView := lipgloss.JoinVertical(lipgloss.Left, "Details", m.details.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/2).Render(deviceView),
		lipgloss.NewStyle().Width(m.width-m.width/2).Render(detailView),
	)
	console := m.console.View()
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

func (m TUIModel) startDiscovery() tea.Cmd {
	if m.dependencies.DiscoverySession == nil {
		return nil
	}
	ctx := m.ctx
	service := m.dependencies.DiscoverySession
	index := m.dependencies.InterfaceIndex
	policy := m.dependencies.DiscoveryPolicy
	return func() tea.Msg {
		session, err := service.Start(ctx, index, policy)
		return tuiDiscoverySessionMsg{session: session, err: err}
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

func (m TUIModel) selectNextInterface() tea.Cmd {
	if len(m.interfaces) == 0 {
		m.status = "no eligible interface available"
		return nil
	}
	next := 0
	if selected, ok := m.SelectedInterface(); ok {
		for index, iface := range m.interfaces {
			if iface.Index == selected.Index {
				next = (index + 1) % len(m.interfaces)
				break
			}
		}
	}
	if err := m.SelectInterface(m.interfaces[next].Index); err != nil {
		m.status = "interface selection failed"
		return nil
	}
	m.status = fmt.Sprintf("selected interface %d; starting discovery", m.interfaces[next].Index)
	return m.startDiscovery()
}

func (m TUIModel) interfaceStatus() string {
	if selected, ok := m.SelectedInterface(); ok {
		return fmt.Sprintf("interface %d (%s)", selected.Index, selected.Name)
	}
	return "interface: none (press i to select)"
}

func (m TUIModel) waitForDiscoveryEvent() tea.Cmd {
	session := m.session
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case device, ok := <-session.Events():
			if !ok {
				return tuiDiscoveryEventMsg{done: true}
			}
			return tuiDiscoveryEventMsg{device: device}
		case err, ok := <-session.Errors():
			if !ok || err == nil {
				return tuiDiscoveryEventMsg{done: true}
			}
			return tuiDiscoveryEventMsg{err: err}
		}
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
			return
		}
	}
	m.discovered = append(m.discovered, converted)
	m.state.SetDevices(m.discovered)
	m.syncDeviceList()
	m.status = fmt.Sprintf("%d device(s) discovered", len(m.discovered))
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
		Model:           device.Model,
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
		items = append(items, deviceItem{device: device})
	}
	m.devices.SetItems(items)
}

func (m *TUIModel) resize() {
	if m.width < 1 || m.height < 1 {
		return
	}
	m.devices.SetSize(m.width/2, m.height-6)
	m.details.Width = m.width - m.width/2
	m.details.Height = m.height - 6
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
	device parser.Device
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
	state := "inactive"
	if value.device.Activated {
		state = "active"
	}
	_, _ = fmt.Fprintf(writer, "%s  %-8s  %s", value.device.MAC, state, sanitizeTUIDisplay(value.device.Model))
}
