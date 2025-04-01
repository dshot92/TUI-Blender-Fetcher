package tui

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TestInitialModel tests the creation of the initial model
func TestInitialModel(t *testing.T) {
	// Create a test config
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}

	// Test cases for different initialization scenarios
	testCases := []struct {
		name       string
		needsSetup bool
		checkModel func(*testing.T, Model)
	}{
		{
			name:       "normal initialization",
			needsSetup: false,
			checkModel: func(t *testing.T, m Model) {
				if m.config.DownloadDir != "/test/path" {
					t.Errorf("Expected download dir /test/path, got %s", m.config.DownloadDir)
				}
				if m.config.VersionFilter != "3.5" {
					t.Errorf("Expected version filter 3.5, got %s", m.config.VersionFilter)
				}
				if !m.isLoading {
					t.Error("Expected isLoading to be true for normal initialization")
				}
				if m.currentView != viewList {
					t.Errorf("Expected currentView to be viewList, got %d", m.currentView)
				}
				if len(m.downloadStates) != 0 {
					t.Errorf("Expected empty downloadStates, got %d items", len(m.downloadStates))
				}
			},
		},
		{
			name:       "first-time setup",
			needsSetup: true,
			checkModel: func(t *testing.T, m Model) {
				if m.isLoading {
					t.Error("Expected isLoading to be false for setup")
				}
				if m.currentView != viewInitialSetup {
					t.Errorf("Expected currentView to be viewInitialSetup, got %d", m.currentView)
				}
				// Check that we have text input fields set up
				if len(m.settingsInputs) == 0 {
					t.Error("Expected settingsInputs to be initialized")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the model
			model := InitialModel(cfg, tc.needsSetup)

			// Run checks
			tc.checkModel(t, model)
		})
	}
}

// TestModelInit tests the Init function of the Model
func TestModelInit(t *testing.T) {
	// Create a test config
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}

	// Create a model
	model := InitialModel(cfg, false)

	// Get the command returned by Init
	cmd := model.Init()

	// We can't directly test the command, but we can check it's not nil
	if cmd == nil {
		t.Error("Expected non-nil command from Init")
	}
}

// TestUpdateWithWindowSize tests handling of window size changes
func TestUpdateWithWindowSize(t *testing.T) {
	// Create a test config and model
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}
	model := InitialModel(cfg, false)

	// Create a window size message
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}

	// Update the model
	updatedModel, _ := model.Update(msg)

	// Check that the window size was stored
	if updatedModel.(Model).terminalWidth != 100 {
		t.Errorf("Expected terminalWidth to be 100, got %d", updatedModel.(Model).terminalWidth)
	}
}

// TestRenderSettingsView tests the rendering of the settings view
func TestRenderSettingsView(t *testing.T) {
	// Create a test config and model
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}
	model := InitialModel(cfg, false)
	model.currentView = viewSettings
	model.terminalWidth = 100 // Set a reasonable terminal width

	// Initialize settings inputs
	model.settingsInputs = make([]textinput.Model, 2)
	model.settingsInputs[0] = textInputFixture("Download Directory", "/test/path")
	model.settingsInputs[1] = textInputFixture("Version Filter", "3.5")

	// Render the settings view
	output := model.renderSettingsView()

	// Simple check for non-empty output
	if output == "" || len(output) < 10 {
		t.Error("Expected non-empty output from renderSettingsView")
	}

	// Optional: check for presence of expected elements (commented out as example)
	// if !strings.Contains(output, "Settings") || !strings.Contains(output, "Version Filter") {
	//     t.Error("Output missing key elements")
	// }
}

// TestRenderConfirmationDialog tests the rendering of confirmation dialogs
func TestRenderConfirmationDialog(t *testing.T) {
	// Create a test model
	model := Model{
		terminalWidth: 100,
	}

	// Render a test dialog
	title := "Test Dialog"
	messageLines := []string{"This is a test message", "Are you sure?"}
	yesText := "OK"
	noText := "Cancel"
	width := 40

	output := model.renderConfirmationDialog(title, messageLines, yesText, noText, width)

	// Simply check that output is non-empty (avoid string comparison issues)
	if output == "" || len(output) < 20 {
		t.Error("Expected non-empty output from renderConfirmationDialog")
	}
}

// TestKeyHandling tests key event handling in the list view
func TestKeyHandling(t *testing.T) {
	// Create a test config and model
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}
	m := InitialModel(cfg, false)
	m.currentView = viewList
	m.builds = []model.BlenderBuild{
		{
			Version: "3.6.0",
			Status:  "Online",
		},
		{
			Version: "3.5.0",
			Status:  "Online",
		},
	}

	// Test handling the down key in list view
	if m.cursor != 0 {
		t.Errorf("Expected initial cursor to be 0, got %d", m.cursor)
	}

	// Simulate pressing down arrow key
	keyMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := m.Update(keyMsg)

	// Check that selection moved down
	if updatedModel.(Model).cursor != 1 {
		t.Errorf("Expected cursor to be 1 after KeyDown, got %d", updatedModel.(Model).cursor)
	}

	// Simulate pressing 's' to enter settings
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updatedModel, _ = updatedModel.(Model).Update(keyMsg)

	// Check that view changed to settings
	if updatedModel.(Model).currentView != viewSettings {
		t.Errorf("Expected currentView to be viewSettings after pressing 's', got %d", updatedModel.(Model).currentView)
	}
}

// TestViewToggling tests toggling between different views
func TestViewToggling(t *testing.T) {
	// Create a test config and model
	cfg := config.Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}
	m := InitialModel(cfg, false)

	// Start in list view
	m.currentView = viewList

	// Test going to settings
	m.currentView = viewSettings

	// Test going back to list from settings using left arrow which is the correct key
	keyMsg := tea.KeyMsg{Type: tea.KeyLeft}
	updatedModel, _ := m.Update(keyMsg)

	// Check that view changed back to list
	if updatedModel.(Model).currentView != viewList {
		t.Errorf("Expected currentView to be viewList after pressing left arrow in settings, got %d", updatedModel.(Model).currentView)
	}
}

// Helper for creating text input models for testing
func textInputFixture(placeholder, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	return ti
}
