package ui

import (
	"errors"
	"net"
	"sort"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
)

func EligibleInterfaces(interfaces []adapter.Interface) []adapter.Interface {
	result := make([]adapter.Interface, 0, len(interfaces))
	for _, iface := range interfaces {
		if isEligibleInterface(iface) {
			result = append(result, iface)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Index < result[j].Index
	})
	return result
}

func ResolveInterface(interfaces []adapter.Interface, index int) (adapter.Interface, error) {
	if index < 1 {
		return adapter.Interface{}, errors.New("interface index must be positive")
	}
	for _, iface := range EligibleInterfaces(interfaces) {
		if iface.Index == index {
			return iface, nil
		}
	}
	return adapter.Interface{}, errors.New("requested interface is unavailable or ineligible")
}

func (m *TUIModel) SetAvailableInterfaces(interfaces []adapter.Interface) {
	if m == nil {
		return
	}
	m.interfaces = EligibleInterfaces(interfaces)
}

func (m *TUIModel) AvailableInterfaces() []adapter.Interface {
	if m == nil {
		return nil
	}
	result := make([]adapter.Interface, len(m.interfaces))
	copy(result, m.interfaces)
	return result
}

func (m *TUIModel) SelectInterface(index int) error {
	if m == nil {
		return errors.New("TUI model is nil")
	}
	selected, err := ResolveInterface(m.interfaces, index)
	if err != nil {
		return err
	}
	value := selected.Index
	m.dependencies.InterfaceIndex = &value
	m.discovered = nil
	m.state.ClearSelection()
	m.state.SetDevices(nil)
	m.syncDeviceList()
	return nil
}

func (m *TUIModel) SelectedInterface() (adapter.Interface, bool) {
	if m == nil || m.dependencies.InterfaceIndex == nil {
		return adapter.Interface{}, false
	}
	selected, err := ResolveInterface(m.interfaces, *m.dependencies.InterfaceIndex)
	if err != nil {
		return adapter.Interface{}, false
	}
	return selected, true
}

func isEligibleInterface(iface adapter.Interface) bool {
	if iface.Index <= 0 || iface.Name == "" || iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return false
	}
	for _, address := range iface.IPv4Addrs {
		if address.IsValid() && address.Addr().Is4() && !address.Addr().IsUnspecified() {
			return true
		}
	}
	return false
}
