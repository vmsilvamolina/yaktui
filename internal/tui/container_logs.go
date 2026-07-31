package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/dockerclient"
)

// ContainerLogsModel represents the logs view for a Docker container. It
// mirrors LogsModel's shape (viewport + follow-toggle + periodic poll) but
// fetches from the Docker daemon instead of the Kubernetes API; the two
// aren't unified into one generic model since their fetch mechanics differ
// enough (different client, different demuxing) that sharing would only
// save the small viewport/poll shell, not real logic. Reuses LogsMsg /
// LogsTickMsg / LogsErrorMsg since those message types carry no pod-specific
// fields.
type ContainerLogsModel struct {
	client        *dockerclient.Client
	containerID   string
	title         string
	viewport      viewport.Model
	logs          string
	err           error
	width         int
	height        int
	loading       bool
	following     bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewContainerLogsModel creates a new container logs model
func NewContainerLogsModel(c *dockerclient.Client, containerID, title string) *ContainerLogsModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Foreground(ColorText)

	ctx, cancel := context.WithCancel(context.Background())
	return &ContainerLogsModel{
		client:      c,
		containerID: containerID,
		title:       title,
		viewport:    vp,
		loading:     true,
		following:   true,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Cancel cancels any in-flight log fetch.
func (m *ContainerLogsModel) Cancel() {
	m.cancel()
}

// Init initializes the model
func (m *ContainerLogsModel) Init() tea.Cmd {
	return m.fetchLogs
}

// SetSize sets the view size
func (m *ContainerLogsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 4
	m.viewport.Height = height - 5
}

// Update handles messages
func (m *ContainerLogsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case LogsErrorMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case LogsMsg:
		m.logs = msg.Logs
		m.loading = false
		m.viewport.SetContent(m.formatLogs(msg.Logs))
		if m.following {
			m.viewport.GotoBottom()
		}
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
func (m *ContainerLogsModel) View() string {
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

	if m.err != nil {
		errText := lipgloss.NewStyle().Foreground(ColorError).Render("✗ " + m.err.Error())
		return header + errText
	}

	return header + m.viewport.View()
}

// GetLogTitle returns the title for the logs view
func (m *ContainerLogsModel) GetLogTitle() string {
	return m.title
}

func (m *ContainerLogsModel) fetchLogs() tea.Msg {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	logs, err := m.client.GetContainerLogs(ctx, m.containerID, 500)
	if err != nil {
		return LogsErrorMsg{Err: err}
	}

	return LogsMsg{Logs: logs}
}

func (m *ContainerLogsModel) formatLogs(logs string) string {
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
		if len(line) > 30 && (line[0] >= '0' && line[0] <= '9') {
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
