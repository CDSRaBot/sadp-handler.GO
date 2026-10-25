package ui

import (
	"errors"
	"fmt"
	"net"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func ResolveTUICommand(state *TUIState, command Command) (Command, error) {
	if state == nil {
		return nil, errors.New("TUI state is nil")
	}
	switch value := command.(type) {
	case ActivateCommand:
		targets, err := resolveTUITargets(state, value.Targets)
		if err != nil {
			return nil, err
		}
		value.Targets = targets
		return value, nil
	case NetworkCommand:
		targets, err := resolveTUITargets(state, value.Targets)
		if err != nil {
			return nil, err
		}
		value.Targets = targets
		return value, nil
	default:
		return command, nil
	}
}

func resolveTUITargets(state *TUIState, selector TargetSelector) (TargetSelector, error) {
	if len(selector.MACs) > 0 {
		if selector.UseTUISelection || selector.UseFocusedDevice {
			return TargetSelector{}, errors.New("target selector strategies conflict")
		}
		return cloneTargetSelector(selector), nil
	}
	if !selector.UseTUISelection && !selector.UseFocusedDevice {
		return TargetSelector{}, errors.New("target selector is empty")
	}
	selected := state.SelectedDevices()
	if len(selected) == 0 {
		focused, ok := state.FocusedDevice()
		if !ok {
			return TargetSelector{}, errors.New("select a device before this operation")
		}
		selected = []parser.Device{focused}
	}
	macs := make([]net.HardwareAddr, 0, len(selected))
	for _, device := range selected {
		if len(device.MAC) != 6 {
			return TargetSelector{}, errors.New("selected device has an invalid MAC")
		}
		macs = append(macs, append(net.HardwareAddr(nil), device.MAC...))
	}
	return TargetSelector{MACs: macs}, nil
}

func cloneTargetSelector(selector TargetSelector) TargetSelector {
	clone := TargetSelector{
		UseTUISelection:  selector.UseTUISelection,
		UseFocusedDevice: selector.UseFocusedDevice,
		MACs:             make([]net.HardwareAddr, 0, len(selector.MACs)),
	}
	for _, mac := range selector.MACs {
		clone.MACs = append(clone.MACs, append(net.HardwareAddr(nil), mac...))
	}
	return clone
}

func confirmationMessage(command Command) string {
	var targets []net.HardwareAddr
	switch value := command.(type) {
	case ActivateCommand:
		targets = value.Targets.MACs
	case NetworkCommand:
		targets = value.Targets.MACs
	default:
		return ""
	}
	if len(targets) < 2 {
		return ""
	}
	return fmt.Sprintf("confirm operation for %d devices (%s): press y/n", len(targets), formatMACs(targets))
}

func formatMACs(macs []net.HardwareAddr) string {
	result := ""
	for index, mac := range macs {
		if index > 0 {
			result += ", "
		}
		result += mac.String()
	}
	return result
}
