package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/victor/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

// LogsModel represents the logs view
type LogsModel struct {
	client        *client.Client
	pod           *corev1.Pod
	containerName string
	viewport      viewport.Model
	logs          string
	width         int
	height        int
	loading       bool
	following     bool
}

// NewLogsModel creates a new logs model
func NewLogsModel(c *client.Client, pod *corev1.Pod) *LogsModel {
	containerName := ""
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Foreground(ColorText)

	return &LogsModel{
		client:        c,
		pod:           pod,
		containerName: containerName,
		viewport:      vp,
		loading:       true,
		following:     true,
	}
}

// Init initializes the model
func (m *LogsModel) Init() tea.Cmd {
	return m.fetchLogs
}

// SetSize sets the view size
func (m *LogsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 4
	m.viewport.Height = height - 5
}

// Update handles messages
func (m *LogsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "g":
			m.viewport.GotoTop()
			return m, nil
		case "G":
			m.viewport.GotoBottom()
			return m, nil
		case "f":
			m.following = !m.following
			return m, nil
		}

	case LogsMsg:
		m.logs = msg.Logs
		m.loading = false
		m.viewport.SetContent(m.formatLogs(msg.Logs))
		if m.following {
			m.viewport.GotoBottom()
		}
		// Schedule next fetch if following
		if m.following {
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return LogsTickMsg{}
			})
		}
		return m, nil

	case LogsTickMsg:
		if m.following {
			return m, m.fetchLogs
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the model
func (m *LogsModel) View() string {
	if m.loading {
		return "Loading logs..."
	}

	followIndicator := ""
	if m.following {
		followIndicator = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Render("[FOLLOWING]")
	} else {
		followIndicator = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("[PAUSED]")
	}

	scrollInfo := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(fmt.Sprintf(" %d%% ", int(m.viewport.ScrollPercent()*100)))

	header := followIndicator + " " + scrollInfo + "\n"

	return header + m.viewport.View()
}

// GetLogTitle returns the title for logs view
func (m *LogsModel) GetLogTitle() string {
	return fmt.Sprintf("%s/%s", m.pod.Name, m.containerName)
}

func (m *LogsModel) fetchLogs() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m.client.SetNamespace(m.pod.Namespace)
	logs, err := m.client.GetPodLogs(ctx, m.pod.Name, m.containerName, 500)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return LogsMsg{Logs: logs}
}

func (m *LogsModel) formatLogs(logs string) string {
	if logs == "" {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No logs available")
	}

	lines := strings.Split(logs, "\n")
	var formatted strings.Builder

	timestampStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Try to highlight timestamps at the beginning of lines
		if len(line) > 30 && (line[0] >= '0' && line[0] <= '9') {
			// Likely starts with a timestamp
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				formatted.WriteString(timestampStyle.Render(parts[0]) + " " + parts[1] + "\n")
				continue
			}
		}
		formatted.WriteString(line + "\n")
	}

	return formatted.String()
}

// Message types
type LogsMsg struct {
	Logs string
}

type LogsTickMsg struct{}
