package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/victor/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

// NamespacesModel represents the namespaces view
type NamespacesModel struct {
	client     *client.Client
	table      table.Model
	namespaces []corev1.Namespace
	width      int
	height     int
	loading    bool
}

// NewNamespacesModel creates a new namespaces model
func NewNamespacesModel(c *client.Client) *NamespacesModel {
	columns := []table.Column{
		{Title: "NAME", Width: 40},
		{Title: "STATUS", Width: 15},
		{Title: "AGE", Width: 10},
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

	return &NamespacesModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *NamespacesModel) Init() tea.Cmd {
	return m.fetchNamespaces
}

// SetSize sets the view size
func (m *NamespacesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAME", Width: totalWidth * 50 / 100},
		{Title: "STATUS", Width: totalWidth * 25 / 100},
		{Title: "AGE", Width: totalWidth * 20 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *NamespacesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case NamespacesMsg:
		m.namespaces = msg.Namespaces
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *NamespacesModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Count returns the number of namespaces
func (m *NamespacesModel) Count() int {
	return len(m.namespaces)
}

// Refresh fetches fresh data
func (m *NamespacesModel) Refresh() tea.Cmd {
	return m.fetchNamespaces
}

// GetSelectedNamespace returns the currently selected namespace
func (m *NamespacesModel) GetSelectedNamespace() string {
	row := m.table.SelectedRow()
	if row == nil {
		return ""
	}
	return row[0]
}

func (m *NamespacesModel) fetchNamespaces() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespaces, err := m.client.ListNamespaces(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return NamespacesMsg{Namespaces: namespaces}
}

func (m *NamespacesModel) updateTable() {
	rows := make([]table.Row, len(m.namespaces))

	for i, ns := range m.namespaces {
		rows[i] = table.Row{
			ns.Name,
			string(ns.Status.Phase),
			formatAge(ns.CreationTimestamp.Time),
		}
	}

	m.table.SetRows(rows)
}

// Message types
type NamespacesMsg struct {
	Namespaces []corev1.Namespace
}
