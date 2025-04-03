package tui

import (
	"strings"
)

func (m *Model) renderPageForView() string {
	// Define fixed heights
	headerHeight := getHeaderHeight() // 1 line
	footerHeight := getFooterHeight() // 2 lines

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
	separator := strings.Repeat(" ", m.terminalWidth)

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
	padding := strings.Repeat("\n", paddingLines)

	// Create the combined view with proper spacing
	return strings.Join([]string{
		header,
		separator,
		content + padding,
		separator,
		footer,
	}, "\n")
}
