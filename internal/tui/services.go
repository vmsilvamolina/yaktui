package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

// ServicesModel represents the services view
type ServicesModel struct {
	client            *client.Client
	table             table.Model
	services          []corev1.Service
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	// Cache for both modes
	servicesAllNS    []corev1.Service
	servicesSingleNS []corev1.Service
	cacheNS          string
}

// NewServicesModel creates a new services model
func NewServicesModel(c *client.Client) *ServicesModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 25},
		{Title: "TYPE", Width: 12},
		{Title: "CLUSTER-IP", Width: 15},
		{Title: "PORTS", Width: 20},
		{Title: "AGE", Width: 8},
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

	return &ServicesModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *ServicesModel) Init() tea.Cmd {
	return m.fetchServices
}

// InitAllNamespaces preloads all namespaces data
func (m *ServicesModel) InitAllNamespaces() tea.Cmd {
	return m.fetchServicesAllNS
}

func (m *ServicesModel) fetchServicesAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svcs, err := m.client.ListAllServices(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ServicesAllNSMsg{Services: svcs}
}

type ServicesAllNSMsg struct {
	Services []corev1.Service
}

// SetShowAllNamespaces sets the all namespaces flag and uses cache if available
func (m *ServicesModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.servicesAllNS) > 0 {
		m.services = m.servicesAllNS
		m.updateTable()
	} else if !showAll && len(m.servicesSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.services = m.servicesSingleNS
		m.updateTable()
	}
}

// SetSize sets the view size
func (m *ServicesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 15 / 100},
		{Title: "NAME", Width: totalWidth * 25 / 100},
		{Title: "TYPE", Width: totalWidth * 12 / 100},
		{Title: "CLUSTER-IP", Width: totalWidth * 15 / 100},
		{Title: "PORTS", Width: totalWidth * 20 / 100},
		{Title: "AGE", Width: totalWidth * 8 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *ServicesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ServicesMsg:
		m.services = msg.Services
		m.loading = false
		if m.showAllNamespaces {
			m.servicesAllNS = msg.Services
		} else {
			m.servicesSingleNS = msg.Services
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case ServicesAllNSMsg:
		m.servicesAllNS = msg.Services
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *ServicesModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Count returns the number of services
func (m *ServicesModel) Count() int {
	return len(m.services)
}

// GetSelectedService returns the currently selected service
func (m *ServicesModel) GetSelectedService() *corev1.Service {
	row := m.table.SelectedRow()
	if row == nil || len(m.services) == 0 {
		return nil
	}
	for i := range m.services {
		if m.services[i].Namespace == row[0] && m.services[i].Name == row[1] {
			return &m.services[i]
		}
	}
	return nil
}

// Refresh fetches fresh data
func (m *ServicesModel) Refresh() tea.Cmd {
	return m.fetchServices
}

func (m *ServicesModel) fetchServices() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var services []corev1.Service
	var err error

	if m.showAllNamespaces {
		services, err = m.client.ListAllServices(ctx)
	} else {
		services, err = m.client.ListServices(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ServicesMsg{Services: services}
}

func (m *ServicesModel) updateTable() {
	rows := make([]table.Row, len(m.services))

	for i, svc := range m.services {
		rows[i] = table.Row{
			svc.Namespace,
			svc.Name,
			string(svc.Spec.Type),
			svc.Spec.ClusterIP,
			formatServicePorts(svc.Spec.Ports),
			formatAge(svc.CreationTimestamp.Time),
		}
	}

	m.table.SetRows(rows)
}

func formatServicePorts(ports []corev1.ServicePort) string {
	var parts []string
	for _, p := range ports {
		if p.NodePort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	return strings.Join(parts, ",")
}

// Message types
type ServicesMsg struct {
	Services []corev1.Service
}
