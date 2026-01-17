package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	ColorPrimary   = lipgloss.Color("39")  // Cyan
	ColorSecondary = lipgloss.Color("212") // Pink
	ColorSuccess   = lipgloss.Color("82")  // Green
	ColorWarning   = lipgloss.Color("214") // Orange
	ColorError     = lipgloss.Color("196") // Red
	ColorMuted     = lipgloss.Color("245") // Gray
	ColorHighlight = lipgloss.Color("226") // Yellow
)

// Styles for various UI elements
var (
	// Text styles
	Bold      = lipgloss.NewStyle().Bold(true)
	Italic    = lipgloss.NewStyle().Italic(true)
	Dim       = lipgloss.NewStyle().Foreground(ColorMuted)
	Highlight = lipgloss.NewStyle().Foreground(ColorHighlight)
	Header    = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// Status styles
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)
	Error   = lipgloss.NewStyle().Foreground(ColorError)

	// Code styles
	Code     = lipgloss.NewStyle().Foreground(ColorPrimary)
	FilePath = lipgloss.NewStyle().Foreground(ColorPrimary)
	LineNum  = lipgloss.NewStyle().Foreground(ColorMuted)

	// Search result styles
	ResultHeader = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
	ResultScore = lipgloss.NewStyle().
			Foreground(ColorSuccess)
	ResultContent = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(2)

	// Section styles
	SectionTitle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			MarginTop(1)
	Divider = lipgloss.NewStyle().
		Foreground(ColorMuted)

	// Citation styles
	Citation = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true)
	SourceRef = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// HorizontalRule returns a styled horizontal divider.
func HorizontalRule(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += "─"
	}
	return Divider.Render(line)
}

// FormatFilePath formats a file path with line numbers.
func FormatFilePath(path string, startLine, endLine int) string {
	return FilePath.Render(path) + LineNum.Render(fmt.Sprintf(":%d-%d", startLine, endLine))
}

// FormatScore formats a similarity score as a percentage.
func FormatScore(score float64) string {
	return ResultScore.Render(fmt.Sprintf("(%.1f%% match)", score*100))
}
