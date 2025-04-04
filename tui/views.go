package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

func (m *Model) renderPageForView() string {
	// Define fixed heights
	headerHeight := 2 // 1 line
	footerHeight := 2 // 2 lines

	// Fixed items: header, footer, 2 separator lines
	fixedHeightItems := headerHeight + footerHeight + 2

	// Calculate content height
	contentHeight := m.terminalHeight - fixedHeightItems
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Generate app components
	header := renderHeader(m.terminalWidth)

	// Create slim horizontal separators
	separatorStyle := lp.NewStyle()
	separator := separatorStyle.Render(strings.Repeat(" ", m.terminalWidth))

	// Generate content and footer based on current view
	var content string
	var footer string

	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		content = m.renderSettingsContent(contentHeight)
		footer = m.renderSettingsFooter()
	} else {
		content = m.renderBuildContent(contentHeight)
		footer = m.renderBuildFooter()
	}

	// Calculate padding needed to push footer to bottom
	renderedContentLines := strings.Count(content, "\n") + 1
	paddingLines := 0
	if renderedContentLines < contentHeight {
		paddingLines = contentHeight - renderedContentLines
	}

	// Use lipgloss to create styled newlines for padding
	paddingStyle := lp.NewStyle()
	padding := paddingStyle.Render(strings.Repeat("\n", paddingLines))

	// Create newline style for joining sections
	newlineStyle := lp.NewStyle().Render("\n")

	// Build the final view with all components properly styled
	var view strings.Builder
	view.WriteString(header)
	view.WriteString(newlineStyle)
	view.WriteString(separator)
	view.WriteString(newlineStyle)
	view.WriteString(content)
	view.WriteString(padding)
	view.WriteString(newlineStyle)
	view.WriteString(separator)
	view.WriteString(newlineStyle)
	view.WriteString(footer)

	return view.String()
}
