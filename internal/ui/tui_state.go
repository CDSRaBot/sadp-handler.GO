package ui

import (
	"errors"
	"net"
	"strings"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

type TUIZone uint8

const (
	TUIZoneDevices TUIZone = iota + 1
	TUIZoneDetails
	TUIZoneConsole
)

type TUIState struct {
	devices       []parser.Device
	filter        string
	focused       int
	selected      map[string]struct{}
	history       []string
	historyCursor int
	zone          TUIZone
	input         string
	detail        string
}

func NewTUIState() *TUIState {
	return &TUIState{
		selected:      make(map[string]struct{}),
		historyCursor: -1,
		zone:          TUIZoneDevices,
	}
}

func (s *TUIState) SetDevices(devices []parser.Device) {
	if s == nil {
		return
	}
	s.devices = append(s.devices[:0], devices...)
	current := make(map[string]struct{}, len(s.devices))
	for _, device := range s.devices {
		current[deviceIdentity(device)] = struct{}{}
	}
	for identity := range s.selected {
		if _, ok := current[identity]; !ok {
			delete(s.selected, identity)
		}
	}
	visible := s.VisibleDevices()
	if len(visible) == 0 {
		s.focused = 0
		return
	}
	if s.focused >= len(visible) {
		s.focused = len(visible) - 1
	}
}

func (s *TUIState) Devices() []parser.Device {
	if s == nil {
		return nil
	}
	return append([]parser.Device(nil), s.devices...)
}

func (s *TUIState) SetFilter(value string) {
	if s == nil {
		return
	}
	s.filter = strings.TrimSpace(value)
	s.focused = 0
}

func (s *TUIState) Filter() string {
	if s == nil {
		return ""
	}
	return s.filter
}

func (s *TUIState) VisibleDevices() []parser.Device {
	if s == nil {
		return []parser.Device{}
	}
	return SearchDevices(s.devices, s.filter)
}

func (s *TUIState) FocusedDevice() (parser.Device, bool) {
	visible := s.VisibleDevices()
	if s == nil || s.focused < 0 || s.focused >= len(visible) {
		return parser.Device{}, false
	}
	return visible[s.focused], true
}

func (s *TUIState) MoveFocus(delta int) {
	if s == nil {
		return
	}
	visible := s.VisibleDevices()
	if len(visible) == 0 || delta == 0 {
		return
	}
	s.focused += delta
	if s.focused < 0 {
		s.focused = 0
	}
	if s.focused >= len(visible) {
		s.focused = len(visible) - 1
	}
}

func (s *TUIState) ToggleFocusedSelection() bool {
	device, ok := s.FocusedDevice()
	if !ok {
		return false
	}
	key := deviceIdentity(device)
	if _, exists := s.selected[key]; exists {
		delete(s.selected, key)
		return false
	}
	s.selected[key] = struct{}{}
	return true
}

func (s *TUIState) SelectedDevices() []parser.Device {
	if s == nil {
		return nil
	}
	result := make([]parser.Device, 0, len(s.selected))
	for _, device := range s.devices {
		if _, ok := s.selected[deviceIdentity(device)]; ok {
			result = append(result, device)
		}
	}
	return result
}

func (s *TUIState) SelectionCount() int {
	if s == nil {
		return 0
	}
	return len(s.selected)
}

func (s *TUIState) ClearSelection() {
	if s == nil {
		return
	}
	clear(s.selected)
}

func (s *TUIState) SetZone(zone TUIZone) error {
	if s == nil {
		return errors.New("TUI state is nil")
	}
	if zone < TUIZoneDevices || zone > TUIZoneConsole {
		return errors.New("invalid TUI zone")
	}
	s.zone = zone
	return nil
}

func (s *TUIState) Zone() TUIZone {
	if s == nil {
		return 0
	}
	return s.zone
}

func (s *TUIState) SetInput(value string) {
	if s != nil {
		s.input = value
	}
}

func (s *TUIState) Input() string {
	if s == nil {
		return ""
	}
	return s.input
}

func (s *TUIState) AddHistory(command string) error {
	if s == nil {
		return errors.New("TUI state is nil")
	}
	if strings.TrimSpace(command) == "" {
		return errors.New("command is empty")
	}
	if strings.Contains(strings.ToLower(command), "--password") {
		return errors.New("sensitive command cannot be stored in history")
	}
	s.history = append(s.history, command)
	s.historyCursor = len(s.history)
	return nil
}

func (s *TUIState) History() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.history...)
}

func (s *TUIState) PreviousHistory() (string, bool) {
	if s == nil || len(s.history) == 0 {
		return "", false
	}
	if s.historyCursor > 0 {
		s.historyCursor--
	}
	return s.history[s.historyCursor], true
}

func (s *TUIState) NextHistory() (string, bool) {
	if s == nil || len(s.history) == 0 {
		return "", false
	}
	if s.historyCursor < len(s.history)-1 {
		s.historyCursor++
		return s.history[s.historyCursor], true
	}
	s.historyCursor = len(s.history)
	return "", true
}

func (s *TUIState) SetDetail(value string) {
	if s != nil {
		s.detail = sanitizeTUIDisplay(value)
	}
}

func (s *TUIState) Detail() string {
	if s == nil {
		return ""
	}
	return s.detail
}

func deviceIdentity(device parser.Device) string {
	sender := ""
	if device.Sender != nil {
		sender = device.Sender.String()
	}
	return strings.ToLower(device.MAC.String()) + "|" + sender
}

func sanitizeTUIDisplay(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' || (r >= ' ' && r != 0x7f) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune(' ')
		}
	}
	return builder.String()
}

func targetSelectorFromState(state *TUIState) TargetSelector {
	if state == nil {
		return TargetSelector{}
	}
	selected := state.SelectedDevices()
	if len(selected) > 0 {
		macs := make([]net.HardwareAddr, 0, len(selected))
		for _, device := range selected {
			macs = append(macs, append(net.HardwareAddr(nil), device.MAC...))
		}
		return TargetSelector{MACs: macs}
	}
	if device, ok := state.FocusedDevice(); ok {
		return TargetSelector{MACs: []net.HardwareAddr{append(net.HardwareAddr(nil), device.MAC...)}}
	}
	return TargetSelector{}
}
