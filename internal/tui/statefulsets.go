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

type StatefulSetsModel struct {
	client            *client.Client
	table             table.Model
	statefulsets      []appsv1.StatefulSet
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	allNSCache        []appsv1.StatefulSet
	singleNSCache     []appsv1.StatefulSet
	cacheNS           string
}

func NewStatefulSetsModel(c *client.Client) *StatefulSetsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "READY", Width: 10},
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

	return &StatefulSetsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

func (m *StatefulSetsModel) Init() tea.Cmd {
	return m.fetchStatefulSets
}

func (m *StatefulSetsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchStatefulSetsAllNS
}

func (m *StatefulSetsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.allNSCache) > 0 {
		m.statefulsets = m.allNSCache
		m.updateTable()
	} else if !showAll && len(m.singleNSCache) > 0 && m.cacheNS == m.client.Namespace() {
		m.statefulsets = m.singleNSCache
		m.updateTable()
	}
}

func (m *StatefulSetsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 20 / 100},
		{Title: "NAME", Width: totalWidth * 40 / 100},
		{Title: "READY", Width: totalWidth * 15 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

func (m *StatefulSetsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case StatefulSetsMsg:
		m.statefulsets = msg.StatefulSets
		m.loading = false
		if m.showAllNamespaces {
			m.allNSCache = msg.StatefulSets
		} else {
			m.singleNSCache = msg.StatefulSets
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case StatefulSetsAllNSMsg:
		m.allNSCache = msg.StatefulSets
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *StatefulSetsModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// SetFilter sets the name filter and re-renders the table
func (m *StatefulSetsModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *StatefulSetsModel) Count() int {
	return m.filteredCount
}

func (m *StatefulSetsModel) Refresh() tea.Cmd {
	return m.fetchStatefulSets
}

func (m *StatefulSetsModel) GetSelectedStatefulSet() *appsv1.StatefulSet {
	row := m.table.SelectedRow()
	if row == nil || len(m.statefulsets) == 0 {
		return nil
	}
	for i := range m.statefulsets {
		if m.statefulsets[i].Namespace == row[0] && m.statefulsets[i].Name == row[1] {
			return &m.statefulsets[i]
		}
	}
	return nil
}

func (m *StatefulSetsModel) fetchStatefulSets() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var sets []appsv1.StatefulSet
	var err error

	if m.showAllNamespaces {
		sets, err = m.client.ListAllStatefulSets(ctx)
	} else {
		sets, err = m.client.ListStatefulSets(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return StatefulSetsMsg{StatefulSets: sets}
}

func (m *StatefulSetsModel) fetchStatefulSetsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sets, err := m.client.ListAllStatefulSets(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return StatefulSetsAllNSMsg{StatefulSets: sets}
}

func (m *StatefulSetsModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, ss := range m.statefulsets {
		if q != "" && !strings.Contains(strings.ToLower(ss.Name), q) {
			continue
		}
		var replicas int32 = 0
		if ss.Spec.Replicas != nil {
			replicas = *ss.Spec.Replicas
		}
		rows = append(rows, table.Row{
			ss.Namespace,
			ss.Name,
			fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, replicas),
			formatAge(ss.CreationTimestamp.Time),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

type StatefulSetsMsg struct {
	StatefulSets []appsv1.StatefulSet
}

type StatefulSetsAllNSMsg struct {
	StatefulSets []appsv1.StatefulSet
}
