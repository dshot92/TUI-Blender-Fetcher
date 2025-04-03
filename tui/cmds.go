// tui/cmds.go
package tui

import (
	"TUI-Blender-Launcher/model"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// These methods on Model are wrappers that use CommandManager to create tea.Cmd commands.

func (m *Model) scanLocalBuildsCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.ScanLocalBuilds()
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(downloadTickRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) uiRefreshCmd() tea.Cmd {
	return func() tea.Msg {
		return forceRenderMsg{}
	}
}

func (m *Model) fetchBuildsCmd() tea.Cmd {
	cm := NewCommandManager(m.config, m.downloadStates, &m.downloadMutex)
	return cm.FetchBuilds()
}

func (m *Model) doDownloadCmd(build model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		return startDownloadMsg{
			build:   build,
			buildID: build.Version + "-" + build.Hash[:8],
		}
	}
}

// adaptiveTickCmd creates a tick command with adaptive rate based on download activity
func (m *Model) adaptiveTickCmd(activeCount int, isExtracting bool) tea.Cmd {
	rate := downloadTickRate

	if activeCount == 0 {
		rate = 500 * time.Millisecond // Slower when idle
	} else if isExtracting {
		rate = 250 * time.Millisecond // During extraction
	} else if activeCount > 1 {
		rate = 80 * time.Millisecond // Multiple downloads
	}

	return tea.Tick(rate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
