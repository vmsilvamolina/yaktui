package tui

import "github.com/charmbracelet/lipgloss"

// Colors for the application
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#00D7FF") // Cyan/Aqua
	ColorSecondary = lipgloss.Color("#FF87D7") // Pink
	ColorAccent    = lipgloss.Color("#FFFF00") // Yellow

	// Status colors
	ColorSuccess = lipgloss.Color("#00FF00") // Green
	ColorWarning = lipgloss.Color("#FFAF00") // Orange
	ColorError   = lipgloss.Color("#FF0000") // Red
	ColorPending = lipgloss.Color("#FFFF00") // Yellow

	// Neutral colors
	ColorBorder      = lipgloss.Color("#444444")
	ColorBorderFocus = lipgloss.Color("#00D7FF")
	ColorMuted       = lipgloss.Color("#666666")
	ColorText        = lipgloss.Color("#FFFFFF")
	ColorSubtle      = lipgloss.Color("#AAAAAA")
)

// Styles for the application
var (
	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginLeft(1)

	// Logo style
	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	// Sidebar styles
	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	SidebarFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorderFocus).
				Padding(0, 1)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				PaddingLeft(1)

	SidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(lipgloss.Color("#005F87")).
				Bold(true).
				PaddingLeft(1)

	SidebarSectionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Bold(true).
				PaddingLeft(1).
				MarginTop(1)

	// Content area styles
	ContentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	ContentFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorderFocus)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TableSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(lipgloss.Color("#005F87")).
				Bold(true)

	// Status bar style
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			Padding(0, 1, 1, 2)

	StatusKeyStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	StatusDescStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	StatusSepStyle = lipgloss.NewStyle()

	// Status indicators
	StatusOKStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	StatusPendingStyle = lipgloss.NewStyle().
				Foreground(ColorPending)

	// Dialog/Modal style
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// StatusText returns styled status text based on status type
func StatusText(status string) string {
	switch status {
	case "Running", "Active", "Ready":
		return StatusOKStyle.Render(status)
	case "Pending", "ContainerCreating", "Terminating":
		return StatusPendingStyle.Render(status)
	case "Failed", "Error", "CrashLoopBackOff":
		return StatusErrorStyle.Render(status)
	default:
		return status
	}
}

// RenderPanelWithTitle renders content in a bordered panel with a title in the top border
func RenderPanelWithTitle(title, content string, width, height int, focused bool) string {
	borderColor := ColorBorder
	if focused {
		borderColor = ColorBorderFocus
	}

	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Calculate inner width (accounting for left/right borders and padding)
	innerWidth := width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Build top border with title: ╭─ Title ─────────╮
	topLeft := borderStyle.Render("╭─ ")
	titleRendered := titleStyle.Render(title)
	topRight := " "

	// Calculate remaining space for dashes
	titleLen := lipgloss.Width(title)
	remainingDashes := innerWidth - titleLen - 1
	if remainingDashes < 0 {
		remainingDashes = 0
	}

	dashes := ""
	for i := 0; i < remainingDashes; i++ {
		dashes += "─"
	}
	topBorder := topLeft + titleRendered + topRight + borderStyle.Render(dashes+"╮")

	// Build content area with side borders
	contentStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(height-2). // Account for top and bottom borders
		Padding(1, 1)     // Add vertical padding to prevent content from sticking to title

	contentLines := contentStyle.Render(content)

	// Add side borders to each line
	lines := splitLines(contentLines)
	var borderedLines string
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		// Pad line to inner width
		lineWidth := lipgloss.Width(line)
		padding := innerWidth - lineWidth
		if padding < 0 {
			padding = 0
		}
		paddedLine := line
		for j := 0; j < padding; j++ {
			paddedLine += " "
		}
		borderedLines += borderStyle.Render("│") + " " + paddedLine + " " + borderStyle.Render("│") + "\n"
	}

	// Build bottom border: ╰─────────────────╯
	bottomDashes := ""
	for i := 0; i < innerWidth+2; i++ {
		bottomDashes += "─"
	}
	bottomBorder := borderStyle.Render("╰" + bottomDashes + "╯")

	return topBorder + "\n" + borderedLines + bottomBorder
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
