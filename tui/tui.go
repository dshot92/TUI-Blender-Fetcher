package tui

import (
	"TUI-Blender-Launcher/api" // Import the api package
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model" // Import the model package
	"TUI-Blender-Launcher/util"  // Import util package
	"fmt"
	"strings" // Import strings

	"github.com/charmbracelet/bubbles/textinput" // Import textinput
	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss" // Import lipgloss
)

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
)

// Define messages for communication between components
type buildsFetchedMsg struct {
	builds []model.BlenderBuild
}
type errMsg struct{ err error }

// Implement the error interface for errMsg
func (e errMsg) Error() string { return e.err.Error() }

// Model represents the state of the TUI application.
type Model struct {
	// Core data
	builds []model.BlenderBuild
	config config.Config

	// UI state
	cursor      int
	isLoading   bool
	err         error
	currentView viewState

	// Settings/Setup specific state
	settingsInputs []textinput.Model // Inputs for download dir, version filter
	focusIndex     int               // Which input is currently focused
}

// Styles using lipgloss
var (
	// Using default terminal colors
	headerStyle = lp.NewStyle().Bold(true).Padding(0, 1)
	// Style for the selected row
	selectedRowStyle = lp.NewStyle().Background(lp.Color("240")).Foreground(lp.Color("255"))
	// Style for regular rows (use default)
	regularRowStyle = lp.NewStyle()
	// Footer style
	footerStyle = lp.NewStyle().MarginTop(1).Faint(true)
	// Separator style (using box characters)
	separator = lp.NewStyle().SetString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━").Faint(true).String()

	// Column Widths (adjust as needed)
	colWidthSelect  = 4 // For "[ ] "
	colWidthVersion = 18
	colWidthStatus  = 18
	colWidthBranch  = 12
	colWidthType    = 18 // Release Cycle
	colWidthHash    = 15
	colWidthSize    = 12
	colWidthDate    = 20 // YYYY-MM-DD HH:MM
)

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) Model {
	m := Model{
		config:    cfg,
		isLoading: !needsSetup, // Don't load builds immediately if setup needed
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		m.settingsInputs = make([]textinput.Model, 2)

		var t textinput.Model
		// Download Dir input
		t = textinput.New()
		t.Placeholder = cfg.DownloadDir // Show default as placeholder
		t.SetValue(cfg.DownloadDir)     // Set initial value
		t.Focus()
		t.CharLimit = 256
		t.Width = 50
		m.settingsInputs[0] = t

		// Version Filter input (renamed from Cutoff)
		t = textinput.New()
		t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
		t.SetValue(cfg.VersionFilter)
		t.CharLimit = 10
		t.Width = 50
		m.settingsInputs[1] = t

		m.focusIndex = 0 // Start focus on the first input
	} else {
		m.currentView = viewList
		// Settings inputs can be initialized when entering settings view
	}

	return m
}

// command to fetch builds
// Now accepts the model to access config
func fetchBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Pass config (specifically VersionFilter) to FetchBuilds
		builds, err := api.FetchBuilds(cfg.VersionFilter)
		if err != nil {
			return errMsg{err}
		}
		return buildsFetchedMsg{builds}
	}
}

// Init initializes the TUI model, potentially running commands.
func (m Model) Init() tea.Cmd {
	if m.currentView == viewList {
		// Fetch initial builds only if starting in list view
		return fetchBuildsCmd(m.config) // Pass config
	}
	// If in setup view, Init might return command to make input blink, etc.
	if len(m.settingsInputs) > 0 {
		return textinput.Blink
	}
	return nil
}

// Helper to update focused input
func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.settingsInputs {
		m.settingsInputs[i], cmds[i] = m.settingsInputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.currentView {
	case viewInitialSetup, viewSettings:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			s := msg.String()
			switch s {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "tab", "shift+tab", "up", "down":
				// Change focus
				oldFocus := m.focusIndex
				if s == "up" || s == "shift+tab" {
					m.focusIndex--
				} else {
					m.focusIndex++
				}
				// Wrap focus
				if m.focusIndex > len(m.settingsInputs)-1 {
					m.focusIndex = 0
				} else if m.focusIndex < 0 {
					m.focusIndex = len(m.settingsInputs) - 1
				}
				// Update focus state on inputs
				for i := 0; i < len(m.settingsInputs); i++ {
					if i == m.focusIndex {
						m.settingsInputs[i].Focus()
						m.settingsInputs[i].PromptStyle = selectedRowStyle // Use a style to indicate focus
					} else {
						m.settingsInputs[i].Blur()
						m.settingsInputs[i].PromptStyle = regularRowStyle
					}
				}
				// If the focus actually changed, update the prompt style of the old one too
				if oldFocus != m.focusIndex && oldFocus >= 0 && oldFocus < len(m.settingsInputs) {
					m.settingsInputs[oldFocus].PromptStyle = regularRowStyle
				}
				return m, nil

			case "enter":
				// Save settings and potentially switch view
				m.config.DownloadDir = m.settingsInputs[0].Value()
				m.config.VersionFilter = m.settingsInputs[1].Value()
				err := config.SaveConfig(m.config)
				if err != nil {
					m.err = fmt.Errorf("failed to save config: %w", err)
					// Stay in settings/setup view on error
				} else {
					m.err = nil
					m.currentView = viewList
					// If coming from initial setup, trigger fetch now
					if !m.isLoading && len(m.builds) == 0 { // Check if fetch is needed
						m.isLoading = true
						cmd = fetchBuildsCmd(m.config) // Pass config
					}
				}
				return m, cmd
			}
		}
		// Pass the message to the focused input
		m.settingsInputs[m.focusIndex], cmd = m.settingsInputs[m.focusIndex].Update(msg)
		return m, cmd

	case viewList:
		switch msg := msg.(type) {
		case buildsFetchedMsg:
			m.isLoading = false
			m.builds = msg.builds
			m.err = nil
			if m.cursor >= len(m.builds) {
				m.cursor = 0
				if len(m.builds) > 0 {
					m.cursor = len(m.builds) - 1
				}
			}
			return m, nil
		case errMsg:
			m.isLoading = false
			m.err = msg.err
			return m, nil
		case tea.KeyMsg:
			if m.isLoading || m.err != nil {
				if msg.String() == "ctrl+c" || msg.String() == "q" {
					return m, tea.Quit
				}
				return m, nil
			}
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if len(m.builds) > 0 && m.cursor < len(m.builds)-1 {
					m.cursor++
				}
			case "f":
				m.isLoading = true
				m.err = nil
				m.builds = nil
				m.cursor = 0
				return m, fetchBuildsCmd(m.config) // Pass config
			case "s": // Enter settings
				m.currentView = viewSettings
				// Initialize settings inputs if not already done
				if len(m.settingsInputs) == 0 {
					m.settingsInputs = make([]textinput.Model, 2)
					var t textinput.Model
					t = textinput.New()
					t.Placeholder = "/path/to/download/dir"
					t.SetValue(m.config.DownloadDir)
					t.Focus()
					t.CharLimit = 256
					t.Width = 50
					m.settingsInputs[0] = t
					t = textinput.New()
					t.Placeholder = "e.g., 4.0, 3.6 (empty for none)"
					t.SetValue(m.config.VersionFilter)
					t.CharLimit = 10
					t.Width = 50
					m.settingsInputs[1] = t
					m.focusIndex = 0
					// Ensure first input has focus style
					m.settingsInputs[0].PromptStyle = selectedRowStyle
				} else {
					// Reset values to current config if re-entering
					m.settingsInputs[0].SetValue(m.config.DownloadDir)
					m.settingsInputs[1].SetValue(m.config.VersionFilter)
					m.focusIndex = 0
					for i := range m.settingsInputs {
						if i == m.focusIndex {
							m.settingsInputs[i].Focus()
							m.settingsInputs[i].PromptStyle = selectedRowStyle
						} else {
							m.settingsInputs[i].Blur()
							m.settingsInputs[i].PromptStyle = regularRowStyle
						}
					}
				}
				return m, textinput.Blink // Start cursor blinking
			}
		}
	}
	return m, cmd // Return potentially updated cmd from list view input handling
}

// View renders the UI based on the model state.
func (m Model) View() string {
	var viewBuilder strings.Builder

	switch m.currentView {
	case viewInitialSetup, viewSettings:
		title := "Initial Setup"
		if m.currentView == viewSettings {
			title = "Settings"
		}
		viewBuilder.WriteString(fmt.Sprintf("%s\n\n", title))
		viewBuilder.WriteString("Download Directory:\n")
		viewBuilder.WriteString(m.settingsInputs[0].View() + "\n\n")
		viewBuilder.WriteString("Minimum Blender Version Filter (e.g., 4.0, 3.6 - empty for none):\n")
		viewBuilder.WriteString(m.settingsInputs[1].View() + "\n\n")
		if m.err != nil {
			viewBuilder.WriteString(lp.NewStyle().Foreground(lp.Color("9")).Render(fmt.Sprintf("Error: %v\n\n", m.err)))
		}
		viewBuilder.WriteString(footerStyle.Render("Tab/Shift+Tab: Change field | Enter: Save | q/Ctrl+C: Quit"))

	case viewList:
		if m.isLoading {
			return "Fetching Blender builds..."
		}
		if m.err != nil {
			return fmt.Sprintf(`Error: %v

Press q to quit.`, m.err)
		}
		if len(m.builds) == 0 {
			return `No Blender builds found matching criteria (check settings/filters).

Press f to fetch again, s for settings, q to quit.`
		}

		// --- Header ---
		headerCols := []string{
			lp.NewStyle().Width(colWidthSelect).Render(""),
			lp.NewStyle().Width(colWidthVersion).Render("Version ↓"),
			lp.NewStyle().Width(colWidthStatus).Render("Status"),
			lp.NewStyle().Width(colWidthBranch).Render("Branch"),
			lp.NewStyle().Width(colWidthType).Render("Type"),
			lp.NewStyle().Width(colWidthHash).Render("Hash"),
			lp.NewStyle().Width(colWidthSize).Render("Size"),
			lp.NewStyle().Width(colWidthDate).Render("Build Date"),
		}
		viewBuilder.WriteString(headerStyle.Render(lp.JoinHorizontal(lp.Left, headerCols...)))
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(separator)
		viewBuilder.WriteString("\n")

		// --- Rows ---
		for i, build := range m.builds {
			selectedMarker := "[ ]"
			if build.Selected {
				selectedMarker = "[x]"
			}
			rowCols := []string{
				lp.NewStyle().Width(colWidthSelect).Render(selectedMarker),
				lp.NewStyle().Width(colWidthVersion).Render(util.TruncateString("Blender "+build.Version, colWidthVersion)),
				lp.NewStyle().Width(colWidthStatus).Render(util.TruncateString(build.Status, colWidthStatus)),
				lp.NewStyle().Width(colWidthBranch).Render(util.TruncateString(build.Branch, colWidthBranch)),
				lp.NewStyle().Width(colWidthType).Render(util.TruncateString(build.ReleaseCycle, colWidthType)),
				lp.NewStyle().Width(colWidthHash).Render(util.TruncateString(build.Hash, colWidthHash)),
				lp.NewStyle().Width(colWidthSize).Align(lp.Right).Render(util.FormatSize(build.Size)),
				lp.NewStyle().Width(colWidthDate).Render(build.BuildDate.Time().Format("2006-01-02 15:04")),
			}
			rowStr := lp.JoinHorizontal(lp.Left, rowCols...)
			if m.cursor == i {
				viewBuilder.WriteString(selectedRowStyle.Render(rowStr))
			} else {
				viewBuilder.WriteString(regularRowStyle.Render(rowStr))
			}
			viewBuilder.WriteString("\n")
		}

		// --- Footer ---
		footerKeybinds1 := "Space:Select  Enter:Launch  D:Download  O:Open Dir  X:Delete"
		footerKeybinds2 := "F:Fetch  R:Reverse  S:Settings  Q:Quit"
		keybindSeparator := "│"
		footerKeys := fmt.Sprintf("%s  %s  %s", footerKeybinds1, keybindSeparator, footerKeybinds2)
		footerLegend := "■ Current Local Version (fetched)   ■ Update Available"
		viewBuilder.WriteString(footerStyle.Render(footerKeys))
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(footerStyle.Render(footerLegend))
	}

	return viewBuilder.String()
}
