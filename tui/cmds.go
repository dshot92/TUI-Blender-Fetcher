// tui/cmds.go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// These methods on Model are wrappers that use Commands to create tea.Cmd commands.

func (m *Model) scanLocalBuildsCmd() tea.Cmd {
	return m.commands.ScanLocalBuilds()
}

func (m *Model) tickCmd() tea.Cmd {
	return m.commands.Tick()
}

func (m *Model) uiRefreshCmd() tea.Cmd {
	return m.commands.UIRefresh()
}

func (m *Model) fetchBuildsCmd() tea.Cmd {
	return m.commands.FetchBuilds()
}
