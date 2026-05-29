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

// NamespacesModel represents the namespaces view
type NamespacesModel struct {
	client        *client.Client
	table         table.Model
	namespaces    []corev1.Namespace
	width         int
	height        int
	loading       bool
	filterQuery   string
	filteredCount int
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
// SetFilter sets the name filter and re-renders the table
func (m *NamespacesModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *NamespacesModel) Count() int {
	return m.filteredCount
}

// Refresh fetches fresh data
func (m *NamespacesModel) Refresh() tea.Cmd {
	return m.fetchNamespaces
}

// GetSelectedNamespace returns the currently selected namespace name
func (m *NamespacesModel) GetSelectedNamespace() string {
	row := m.table.SelectedRow()
	if row == nil {
		return ""
	}
	return row[0]
}

// GetSelectedNamespaceObj returns the currently selected namespace object
func (m *NamespacesModel) GetSelectedNamespaceObj() *corev1.Namespace {
	row := m.table.SelectedRow()
	if row == nil || len(m.namespaces) == 0 {
		return nil
	}
	for i := range m.namespaces {
		if m.namespaces[i].Name == row[0] {
			return &m.namespaces[i]
		}
	}
	return nil
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
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, ns := range m.namespaces {
		if q != "" && !strings.Contains(strings.ToLower(ns.Name), q) {
			continue
		}
		rows = append(rows, table.Row{
			ns.Name,
			string(ns.Status.Phase),
			formatAge(ns.CreationTimestamp.Time),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// Message types
type NamespacesMsg struct {
	Namespaces []corev1.Namespace
}
