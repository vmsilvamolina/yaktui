package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

type NodesModel struct {
	client  *client.Client
	table   table.Model
	nodes   []corev1.Node
	width   int
	height  int
	loading bool
}

func NewNodesModel(c *client.Client) *NodesModel {
	columns := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "STATUS", Width: 12},
		{Title: "ROLES", Width: 20},
		{Title: "AGE", Width: 8},
		{Title: "VERSION", Width: 16},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		MarginTop(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(ColorAccent)
	s.Selected = s.Selected.
		Foreground(ColorText).
		Background(lipgloss.Color("#005F87")).
		Bold(true)
	t.SetStyles(s)

	return &NodesModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

func (m *NodesModel) Init() tea.Cmd {
	return m.fetchNodes
}

func (m *NodesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAME", Width: totalWidth * 30 / 100},
		{Title: "STATUS", Width: totalWidth * 12 / 100},
		{Title: "ROLES", Width: totalWidth * 20 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
		{Title: "VERSION", Width: totalWidth * 18 / 100},
	}
	m.table.SetColumns(columns)
}

func (m *NodesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case NodesMsg:
		m.nodes = msg.Nodes
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *NodesModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

func (m *NodesModel) Count() int {
	return len(m.nodes)
}

func (m *NodesModel) Refresh() tea.Cmd {
	return m.fetchNodes
}

func (m *NodesModel) GetSelectedNode() *corev1.Node {
	row := m.table.SelectedRow()
	if row == nil || len(m.nodes) == 0 {
		return nil
	}
	for i := range m.nodes {
		if m.nodes[i].Name == row[0] {
			return &m.nodes[i]
		}
	}
	return nil
}

func (m *NodesModel) fetchNodes() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodes, err := m.client.ListNodes(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return NodesMsg{Nodes: nodes}
}

func (m *NodesModel) updateTable() {
	rows := make([]table.Row, len(m.nodes))

	for i, node := range m.nodes {
		rows[i] = table.Row{
			node.Name,
			nodeStatus(node),
			nodeRoles(node),
			formatAge(node.CreationTimestamp.Time),
			node.Status.NodeInfo.KubeletVersion,
		}
	}

	m.table.SetRows(rows)
}

func nodeStatus(node corev1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(node corev1.Node) string {
	var roles []string
	for label := range node.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

type NodesMsg struct {
	Nodes []corev1.Node
}
