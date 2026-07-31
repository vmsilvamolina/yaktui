package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types/container"
	"github.com/vmsilvamolina/yaktui/internal/dockerclient"
)

// ContainersModel represents the Docker containers view
type ContainersModel struct {
	client        *dockerclient.Client
	table         table.Model
	containers    []container.Summary
	width         int
	height        int
	loading       bool
	filterQuery   string
	filteredCount int
}

// NewContainersModel creates a new containers model
func NewContainersModel(c *dockerclient.Client) *ContainersModel {
	columns := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "IMAGE", Width: 25},
		{Title: "STATUS", Width: 20},
		{Title: "PORTS", Width: 20},
		{Title: "CREATED", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(podTableStylesNormal())

	return &ContainersModel{client: c, table: t, loading: true}
}

// Init initializes the model
func (m *ContainersModel) Init() tea.Cmd {
	return m.fetchContainers
}

// SetSize sets the view size
func (m *ContainersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAME", Width: totalWidth * 25 / 100},
		{Title: "IMAGE", Width: totalWidth * 25 / 100},
		{Title: "STATUS", Width: totalWidth * 20 / 100},
		{Title: "PORTS", Width: totalWidth * 20 / 100},
		{Title: "CREATED", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *ContainersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ContainersMsg:
		m.containers = msg.Containers
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *ContainersModel) View() string {
	if m.loading {
		return "Loading..."
	}

	row := m.table.SelectedRow()
	if len(row) > 2 {
		status := row[2]
		switch {
		case strings.HasPrefix(status, "Up"):
			m.table.SetStyles(podTableStylesNormal())
		case strings.HasPrefix(status, "Exited (0)"):
			m.table.SetStyles(podTableStylesWarning())
		case strings.HasPrefix(status, "Exited"):
			m.table.SetStyles(podTableStylesError())
		default:
			m.table.SetStyles(podTableStylesWarning())
		}
	}

	return m.table.View()
}

// Refresh fetches fresh data
func (m *ContainersModel) Refresh() tea.Cmd {
	return m.fetchContainers
}

// SetFilter sets the name filter and re-renders the table
func (m *ContainersModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

// Count returns the number of displayed containers
func (m *ContainersModel) Count() int {
	return m.filteredCount
}

// GetSelectedContainer returns the currently selected container
func (m *ContainersModel) GetSelectedContainer() *container.Summary {
	row := m.table.SelectedRow()
	if row == nil || len(m.containers) == 0 {
		return nil
	}
	for i := range m.containers {
		if containerDisplayName(m.containers[i]) == row[0] {
			return &m.containers[i]
		}
	}
	return nil
}

func (m *ContainersModel) fetchContainers() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containers, err := m.client.ListContainers(ctx, true)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ContainersMsg{Containers: containers}
}

func (m *ContainersModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, c := range m.containers {
		name := containerDisplayName(c)
		if q != "" && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		rows = append(rows, table.Row{
			name,
			c.Image,
			c.Status,
			formatContainerPorts(c.Ports),
			formatAge(time.Unix(c.Created, 0)),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// containerDisplayName returns a container's primary name with the Docker
// API's leading "/" stripped, or a short ID if it has no name.
func containerDisplayName(c container.Summary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

func formatContainerPorts(ports []container.Port) string {
	var parts []string
	for _, p := range ports {
		if p.PublicPort != 0 {
			parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}
	return strings.Join(parts, ", ")
}

// Message types
type ContainersMsg struct {
	Containers []container.Summary
}
