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
	appsv1 "k8s.io/api/apps/v1"
)

// DeploymentsModel represents the deployments view
type DeploymentsModel struct {
	client            *client.Client
	table             table.Model
	deployments       []appsv1.Deployment
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	// Cache for both modes
	deploymentsAllNS    []appsv1.Deployment
	deploymentsSingleNS []appsv1.Deployment
	cacheNS             string
}

// NewDeploymentsModel creates a new deployments model
func NewDeploymentsModel(c *client.Client) *DeploymentsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "READY", Width: 10},
		{Title: "UP-TO-DATE", Width: 12},
		{Title: "AVAILABLE", Width: 12},
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

	return &DeploymentsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *DeploymentsModel) Init() tea.Cmd {
	return m.fetchDeployments
}

// InitAllNamespaces preloads all namespaces data
func (m *DeploymentsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchDeploymentsAllNS
}

func (m *DeploymentsModel) fetchDeploymentsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deps, err := m.client.ListAllDeployments(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return DeploymentsAllNSMsg{Deployments: deps}
}

type DeploymentsAllNSMsg struct {
	Deployments []appsv1.Deployment
}

// SetShowAllNamespaces sets the all namespaces flag and uses cache if available
func (m *DeploymentsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.deploymentsAllNS) > 0 {
		m.deployments = m.deploymentsAllNS
		m.updateTable()
	} else if !showAll && len(m.deploymentsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.deployments = m.deploymentsSingleNS
		m.updateTable()
	}
}

// SetSize sets the view size
func (m *DeploymentsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 15 / 100},
		{Title: "NAME", Width: totalWidth * 30 / 100},
		{Title: "READY", Width: totalWidth * 12 / 100},
		{Title: "UP-TO-DATE", Width: totalWidth * 15 / 100},
		{Title: "AVAILABLE", Width: totalWidth * 12 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *DeploymentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case DeploymentsMsg:
		m.deployments = msg.Deployments
		m.loading = false
		if m.showAllNamespaces {
			m.deploymentsAllNS = msg.Deployments
		} else {
			m.deploymentsSingleNS = msg.Deployments
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case DeploymentsAllNSMsg:
		m.deploymentsAllNS = msg.Deployments
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *DeploymentsModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Count returns the number of deployments
// SetFilter sets the name filter and re-renders the table
func (m *DeploymentsModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *DeploymentsModel) Count() int {
	return m.filteredCount
}

// GetSelectedDeployment returns the currently selected deployment
func (m *DeploymentsModel) GetSelectedDeployment() *appsv1.Deployment {
	row := m.table.SelectedRow()
	if row == nil || len(m.deployments) == 0 {
		return nil
	}
	for i := range m.deployments {
		if m.deployments[i].Namespace == row[0] && m.deployments[i].Name == row[1] {
			return &m.deployments[i]
		}
	}
	return nil
}

// Refresh fetches fresh data
func (m *DeploymentsModel) Refresh() tea.Cmd {
	return m.fetchDeployments
}

func (m *DeploymentsModel) fetchDeployments() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var deployments []appsv1.Deployment
	var err error

	if m.showAllNamespaces {
		deployments, err = m.client.ListAllDeployments(ctx)
	} else {
		deployments, err = m.client.ListDeployments(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return DeploymentsMsg{Deployments: deployments}
}

func (m *DeploymentsModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, dep := range m.deployments {
		if q != "" && !strings.Contains(strings.ToLower(dep.Name), q) {
			continue
		}
		var replicas int32 = 0
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		rows = append(rows, table.Row{
			dep.Namespace,
			dep.Name,
			fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, replicas),
			fmt.Sprintf("%d", dep.Status.UpdatedReplicas),
			fmt.Sprintf("%d", dep.Status.AvailableReplicas),
			formatAge(dep.CreationTimestamp.Time),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// Message types
type DeploymentsMsg struct {
	Deployments []appsv1.Deployment
}
