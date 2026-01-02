package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/victor/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

// ConfigMapsModel represents the configmaps view
type ConfigMapsModel struct {
	client            *client.Client
	table             table.Model
	configmaps        []corev1.ConfigMap
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	// Cache for both modes
	configmapsAllNS    []corev1.ConfigMap
	configmapsSingleNS []corev1.ConfigMap
	cacheNS            string
}

// NewConfigMapsModel creates a new configmaps model
func NewConfigMapsModel(c *client.Client) *ConfigMapsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 20},
		{Title: "NAME", Width: 40},
		{Title: "DATA", Width: 10},
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

	return &ConfigMapsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *ConfigMapsModel) Init() tea.Cmd {
	return m.fetchConfigMaps
}

// InitAllNamespaces preloads all namespaces data
func (m *ConfigMapsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchConfigMapsAllNS
}

func (m *ConfigMapsModel) fetchConfigMapsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cms, err := m.client.ListAllConfigMaps(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ConfigMapsAllNSMsg{ConfigMaps: cms}
}

type ConfigMapsAllNSMsg struct {
	ConfigMaps []corev1.ConfigMap
}

// SetShowAllNamespaces sets the all namespaces flag and uses cache if available
func (m *ConfigMapsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.configmapsAllNS) > 0 {
		m.configmaps = m.configmapsAllNS
		m.updateTable()
	} else if !showAll && len(m.configmapsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.configmaps = m.configmapsSingleNS
		m.updateTable()
	}
}

// SetSize sets the view size
func (m *ConfigMapsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 20 / 100},
		{Title: "NAME", Width: totalWidth * 50 / 100},
		{Title: "DATA", Width: totalWidth * 15 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *ConfigMapsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ConfigMapsMsg:
		m.configmaps = msg.ConfigMaps
		m.loading = false
		if m.showAllNamespaces {
			m.configmapsAllNS = msg.ConfigMaps
		} else {
			m.configmapsSingleNS = msg.ConfigMaps
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case ConfigMapsAllNSMsg:
		m.configmapsAllNS = msg.ConfigMaps
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *ConfigMapsModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Count returns the number of configmaps
func (m *ConfigMapsModel) Count() int {
	return len(m.configmaps)
}

// Refresh fetches fresh data
func (m *ConfigMapsModel) Refresh() tea.Cmd {
	return m.fetchConfigMaps
}

func (m *ConfigMapsModel) fetchConfigMaps() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var configmaps []corev1.ConfigMap
	var err error

	if m.showAllNamespaces {
		configmaps, err = m.client.ListAllConfigMaps(ctx)
	} else {
		configmaps, err = m.client.ListConfigMaps(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ConfigMapsMsg{ConfigMaps: configmaps}
}

func (m *ConfigMapsModel) updateTable() {
	rows := make([]table.Row, len(m.configmaps))

	for i, cm := range m.configmaps {
		rows[i] = table.Row{
			cm.Namespace,
			cm.Name,
			fmt.Sprintf("%d", len(cm.Data)),
			formatAge(cm.CreationTimestamp.Time),
		}
	}

	m.table.SetRows(rows)
}

// Message types
type ConfigMapsMsg struct {
	ConfigMaps []corev1.ConfigMap
}
