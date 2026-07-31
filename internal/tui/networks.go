package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types/network"
	"github.com/vmsilvamolina/yaktui/internal/dockerclient"
)

// NetworksModel represents the Docker networks view
type NetworksModel struct {
	client        *dockerclient.Client
	table         table.Model
	networks      []network.Summary
	width         int
	height        int
	loading       bool
	filterQuery   string
	filteredCount int
}

// NewNetworksModel creates a new networks model
func NewNetworksModel(c *dockerclient.Client) *NetworksModel {
	columns := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "DRIVER", Width: 15},
		{Title: "SCOPE", Width: 10},
		{Title: "AGE", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(podTableStylesNormal())

	return &NetworksModel{client: c, table: t, loading: true}
}

// Init initializes the model
func (m *NetworksModel) Init() tea.Cmd {
	return m.fetchNetworks
}

// SetSize sets the view size
func (m *NetworksModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAME", Width: totalWidth * 40 / 100},
		{Title: "DRIVER", Width: totalWidth * 25 / 100},
		{Title: "SCOPE", Width: totalWidth * 15 / 100},
		{Title: "AGE", Width: totalWidth * 20 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *NetworksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case NetworksMsg:
		m.networks = msg.Networks
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *NetworksModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Refresh fetches fresh data
func (m *NetworksModel) Refresh() tea.Cmd {
	return m.fetchNetworks
}

// SetFilter sets the name filter and re-renders the table
func (m *NetworksModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

// Count returns the number of displayed networks
func (m *NetworksModel) Count() int {
	return m.filteredCount
}

// GetSelectedNetwork returns the currently selected network
func (m *NetworksModel) GetSelectedNetwork() *network.Summary {
	row := m.table.SelectedRow()
	if row == nil || len(m.networks) == 0 {
		return nil
	}
	for i := range m.networks {
		if m.networks[i].Name == row[0] {
			return &m.networks[i]
		}
	}
	return nil
}

func (m *NetworksModel) fetchNetworks() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	networks, err := m.client.ListNetworks(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return NetworksMsg{Networks: networks}
}

func (m *NetworksModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, n := range m.networks {
		if q != "" && !strings.Contains(strings.ToLower(n.Name), q) {
			continue
		}
		rows = append(rows, table.Row{
			n.Name,
			n.Driver,
			n.Scope,
			formatAge(n.Created),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// Message types
type NetworksMsg struct {
	Networks []network.Summary
}
