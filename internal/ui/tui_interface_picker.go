package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m TUIModel) handleInterfacePicker(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc":
		m.interfacePicker = false
		m.status = "interface selection canceled"
	case "up", "k":
		if m.interfaceCursor > 0 {
			m.interfaceCursor--
		}
	case "down", "j":
		if m.interfaceCursor+1 < len(m.interfaces) {
			m.interfaceCursor++
		}
	case "enter":
		if m.interfaceCursor < 0 || m.interfaceCursor >= len(m.interfaces) {
			m.status = "interface selection is unavailable"
			return m, nil
		}
		index := m.interfaces[m.interfaceCursor].Index
		if err := m.SelectInterface(index); err != nil {
			m.status = "interface selection failed"
			return m, nil
		}
		m.interfacePicker = false
		return m, m.startDiscovery()
	}
	return m, nil
}

func (m TUIModel) interfacePickerView() string {
	var lines []string
	lines = append(lines, "Choose network interface (arrows, Enter, Esc)")
	limit := m.height - 6
	if limit < 1 {
		limit = 1
	}
	start := 0
	if m.interfaceCursor >= limit {
		start = m.interfaceCursor - limit + 1
	}
	end := start + limit
	if end > len(m.interfaces) {
		end = len(m.interfaces)
	}
	for index := start; index < end; index++ {
		iface := m.interfaces[index]
		cursor := " "
		if index == m.interfaceCursor {
			cursor = ">"
		}
		addresses := make([]string, 0, len(iface.IPv4Addrs))
		for _, address := range iface.IPv4Addrs {
			addresses = append(addresses, address.String())
		}
		lines = append(lines, fmt.Sprintf("%s %d  %s  %s", cursor, iface.Index,
			sanitizeTUIDisplay(iface.Name), strings.Join(addresses, ", ")))
	}
	width := m.width - 2
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(width).
		Render(strings.Join(lines, "\n"))
}
