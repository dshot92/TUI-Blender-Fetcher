package tui

import (
	"TUI-Blender-Launcher/api"   // Import the api package
	"TUI-Blender-Launcher/model" // Import the model package
	"TUI-Blender-Launcher/util"  // Import util package
	"fmt"
	"strings" // Import strings

	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss" // Import lipgloss
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
	builds    []model.BlenderBuild
	cursor    int
	isLoading bool
	err       error
	// Add other state fields like selected items, view states (list/settings) etc.
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
func InitialModel() Model {
	return Model{
		isLoading: true, // Start in loading state
	}
}

// command to fetch builds
func fetchBuildsCmd() tea.Msg {
	builds, err := api.FetchBuilds()
	if err != nil {
		// On error, return an errMsg
		return errMsg{err}
	}
	// On success, return a buildsFetchedMsg
	return buildsFetchedMsg{builds}
}

// Init initializes the TUI model, potentially running commands.
func (m Model) Init() tea.Cmd {
	// Fetch initial builds when the TUI starts
	return fetchBuildsCmd
}

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Handle fetched builds
	case buildsFetchedMsg:
		m.isLoading = false
		m.builds = msg.builds
		m.err = nil // Clear previous errors
		// Reset cursor if out of bounds after fetch
		if m.cursor >= len(m.builds) {
			m.cursor = 0
			if len(m.builds) > 0 {
				m.cursor = len(m.builds) - 1
			}
		}
		return m, nil

	// Handle error during fetch
	case errMsg:
		m.isLoading = false
		m.err = msg.err
		return m, nil

	// Handle key presses
	case tea.KeyMsg:
		// Don't process keys if loading or error state (allow quit)
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
			if len(m.builds) > 0 && m.cursor < len(m.builds)-1 { // Check bounds
				m.cursor++
			}

		case "f": // Fetch builds again
			m.isLoading = true
			m.err = nil
			m.builds = nil
			m.cursor = 0
			return m, fetchBuildsCmd

		case " ": // Select/deselect
			if len(m.builds) > 0 && m.cursor >= 0 && m.cursor < len(m.builds) {
				m.builds[m.cursor].Selected = !m.builds[m.cursor].Selected
			}

			// TODO: Handle other keys (enter, d, s, o, r, x)

		}
	}

	return m, nil
}

// View renders the UI based on the model state.
func (m Model) View() string {
	if m.isLoading {
		return "Fetching Blender builds..."
	}

	if m.err != nil {
		return fmt.Sprintf(`Error fetching builds: %v

Press q to quit.`, m.err)
	}

	if len(m.builds) == 0 {
		return `No Blender builds found for your OS/Arch.

Press f to fetch again, q to quit.`
	}

	var viewBuilder strings.Builder

	// --- Header ---
	headerCols := []string{
		lp.NewStyle().Width(colWidthSelect).Render(""), // Spacer for select
		lp.NewStyle().Width(colWidthVersion).Render("Version ↓"),
		lp.NewStyle().Width(colWidthStatus).Render("Status"),
		lp.NewStyle().Width(colWidthBranch).Render("Branch"),
		lp.NewStyle().Width(colWidthType).Render("Type"), // Release Cycle
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

		// Format columns for the current build
		rowCols := []string{
			lp.NewStyle().Width(colWidthSelect).Render(selectedMarker),
			lp.NewStyle().Width(colWidthVersion).Render(util.TruncateString("Blender "+build.Version, colWidthVersion)),
			lp.NewStyle().Width(colWidthStatus).Render(util.TruncateString(build.Status, colWidthStatus)), // TODO: Enhance status (Downloaded, Update)
			lp.NewStyle().Width(colWidthBranch).Render(util.TruncateString(build.Branch, colWidthBranch)),
			lp.NewStyle().Width(colWidthType).Render(util.TruncateString(build.ReleaseCycle, colWidthType)),
			lp.NewStyle().Width(colWidthHash).Render(util.TruncateString(build.Hash, colWidthHash)),
			lp.NewStyle().Width(colWidthSize).Align(lp.Right).Render(util.FormatSize(build.Size)),       // Align size right
			lp.NewStyle().Width(colWidthDate).Render(build.BuildDate.Time().Format("2006-01-02 15:04")), // Format date
		}

		rowStr := lp.JoinHorizontal(lp.Left, rowCols...)

		// Apply style based on cursor position
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

	// Combine keybind parts - ensure they fit within terminal width (lipgloss can help with wrapping/truncating too)
	// Simple joining for now:
	footerKeys := fmt.Sprintf("%s  %s  %s", footerKeybinds1, keybindSeparator, footerKeybinds2)

	footerLegend := "■ Current Local Version (fetched)   ■ Update Available" // TODO: Implement logic for these indicators

	viewBuilder.WriteString(footerStyle.Render(footerKeys))
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(footerStyle.Render(footerLegend))

	return viewBuilder.String()
}
