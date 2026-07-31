package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types/volume"
	"github.com/vmsilvamolina/yaktui/internal/dockerclient"
)

// VolumesModel represents the Docker volumes view
type VolumesModel struct {
	client        *dockerclient.Client
	table         table.Model
	volumes       []*volume.Volume
	width         int
	height        int
	loading       bool
	filterQuery   string
	filteredCount int
}

// NewVolumesModel creates a new volumes model
func NewVolumesModel(c *dockerclient.Client) *VolumesModel {
	columns := []table.Column{
		{Title: "NAME", Width: 35},
		{Title: "DRIVER", Width: 15},
		{Title: "MOUNTPOINT", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(podTableStylesNormal())

	return &VolumesModel{client: c, table: t, loading: true}
}

// Init initializes the model
func (m *VolumesModel) Init() tea.Cmd {
	return m.fetchVolumes
}

// SetSize sets the view size
func (m *VolumesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAME", Width: totalWidth * 30 / 100},
		{Title: "DRIVER", Width: totalWidth * 20 / 100},
		{Title: "MOUNTPOINT", Width: totalWidth * 50 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *VolumesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case VolumesMsg:
		m.volumes = msg.Volumes
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *VolumesModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Refresh fetches fresh data
func (m *VolumesModel) Refresh() tea.Cmd {
	return m.fetchVolumes
}

// SetFilter sets the name filter and re-renders the table
func (m *VolumesModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

// Count returns the number of displayed volumes
func (m *VolumesModel) Count() int {
	return m.filteredCount
}

// GetSelectedVolume returns the currently selected volume
func (m *VolumesModel) GetSelectedVolume() *volume.Volume {
	row := m.table.SelectedRow()
	if row == nil || len(m.volumes) == 0 {
		return nil
	}
	for _, v := range m.volumes {
		if v.Name == row[0] {
			return v
		}
	}
	return nil
}

func (m *VolumesModel) fetchVolumes() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	volumes, err := m.client.ListVolumes(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return VolumesMsg{Volumes: volumes}
}

func (m *VolumesModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, v := range m.volumes {
		if q != "" && !strings.Contains(strings.ToLower(v.Name), q) {
			continue
		}
		rows = append(rows, table.Row{
			v.Name,
			v.Driver,
			v.Mountpoint,
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// Message types
type VolumesMsg struct {
	Volumes []*volume.Volume
}
