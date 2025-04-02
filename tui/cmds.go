// tui/cmds.go
package tui

import (
	"TUI-Blender-Launcher/model"

	tea "github.com/charmbracelet/bubbletea"
)

// These methods on Model are wrappers that use CommandManager to create tea.Cmd commands.

func (m *Model) scanLocalBuildsCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.ScanLocalBuilds()
}

func (m *Model) tickCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.Tick()
}

func (m *Model) uiRefreshCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.UIRefresh()
}

func (m *Model) fetchBuildsCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.FetchBuilds()
}

func (m *Model) doDownloadCmd(build model.BlenderBuild) tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.DoDownload(build)
}
