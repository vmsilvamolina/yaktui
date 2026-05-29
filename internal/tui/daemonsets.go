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

type DaemonSetsModel struct {
	client            *client.Client
	table             table.Model
	daemonsets        []appsv1.DaemonSet
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	allNSCache        []appsv1.DaemonSet
	singleNSCache     []appsv1.DaemonSet
	cacheNS           string
}

func NewDaemonSetsModel(c *client.Client) *DaemonSetsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "DESIRED", Width: 9},
		{Title: "CURRENT", Width: 9},
		{Title: "READY", Width: 8},
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

	return &DaemonSetsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

func (m *DaemonSetsModel) Init() tea.Cmd {
	return m.fetchDaemonSets
}

func (m *DaemonSetsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchDaemonSetsAllNS
}

func (m *DaemonSetsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.allNSCache) > 0 {
		m.daemonsets = m.allNSCache
		m.updateTable()
	} else if !showAll && len(m.singleNSCache) > 0 && m.cacheNS == m.client.Namespace() {
		m.daemonsets = m.singleNSCache
		m.updateTable()
	}
}

func (m *DaemonSetsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 20 / 100},
		{Title: "NAME", Width: totalWidth * 35 / 100},
		{Title: "DESIRED", Width: totalWidth * 10 / 100},
		{Title: "CURRENT", Width: totalWidth * 10 / 100},
		{Title: "READY", Width: totalWidth * 10 / 100},
		{Title: "AGE", Width: totalWidth * 8 / 100},
	}
	m.table.SetColumns(columns)
}

func (m *DaemonSetsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case DaemonSetsMsg:
		m.daemonsets = msg.DaemonSets
		m.loading = false
		if m.showAllNamespaces {
			m.allNSCache = msg.DaemonSets
		} else {
			m.singleNSCache = msg.DaemonSets
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case DaemonSetsAllNSMsg:
		m.allNSCache = msg.DaemonSets
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *DaemonSetsModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// SetFilter sets the name filter and re-renders the table
func (m *DaemonSetsModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *DaemonSetsModel) Count() int {
	return m.filteredCount
}

func (m *DaemonSetsModel) Refresh() tea.Cmd {
	return m.fetchDaemonSets
}

func (m *DaemonSetsModel) GetSelectedDaemonSet() *appsv1.DaemonSet {
	row := m.table.SelectedRow()
	if row == nil || len(m.daemonsets) == 0 {
		return nil
	}
	for i := range m.daemonsets {
		if m.daemonsets[i].Namespace == row[0] && m.daemonsets[i].Name == row[1] {
			return &m.daemonsets[i]
		}
	}
	return nil
}

func (m *DaemonSetsModel) fetchDaemonSets() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var sets []appsv1.DaemonSet
	var err error

	if m.showAllNamespaces {
		sets, err = m.client.ListAllDaemonSets(ctx)
	} else {
		sets, err = m.client.ListDaemonSets(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return DaemonSetsMsg{DaemonSets: sets}
}

func (m *DaemonSetsModel) fetchDaemonSetsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sets, err := m.client.ListAllDaemonSets(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return DaemonSetsAllNSMsg{DaemonSets: sets}
}

func (m *DaemonSetsModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, ds := range m.daemonsets {
		if q != "" && !strings.Contains(strings.ToLower(ds.Name), q) {
			continue
		}
		rows = append(rows, table.Row{
			ds.Namespace,
			ds.Name,
			fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
			fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled),
			fmt.Sprintf("%d", ds.Status.NumberReady),
			formatAge(ds.CreationTimestamp.Time),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

type DaemonSetsMsg struct {
	DaemonSets []appsv1.DaemonSet
}

type DaemonSetsAllNSMsg struct {
	DaemonSets []appsv1.DaemonSet
}
