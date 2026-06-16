package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/client"
	corev1 "k8s.io/api/core/v1"
)

type EventsModel struct {
	client            *client.Client
	table             table.Model
	events            []corev1.Event
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	eventsAllNS       []corev1.Event
	eventsSingleNS    []corev1.Event
	cacheNS           string
}

func eventTableStylesNormal() table.Styles {
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
	return s
}

func eventTableStylesWarning() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		MarginTop(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(ColorAccent)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#FFAF00")).
		Bold(true)
	return s
}

func NewEventsModel(c *client.Client) *EventsModel {
	columns := []table.Column{
		{Title: "AGE", Width: 6},
		{Title: "TYPE", Width: 8},
		{Title: "REASON", Width: 18},
		{Title: "OBJECT", Width: 30},
		{Title: "NAMESPACE", Width: 15},
		{Title: "COUNT", Width: 6},
		{Title: "MESSAGE", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(eventTableStylesNormal())

	return &EventsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

func (m *EventsModel) Init() tea.Cmd {
	return m.fetchEvents
}

func (m *EventsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchEventsAllNS
}

func (m *EventsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.eventsAllNS) > 0 {
		m.events = m.eventsAllNS
		m.updateTable()
	} else if !showAll && len(m.eventsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.events = m.eventsSingleNS
		m.updateTable()
	}
}

func (m *EventsModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *EventsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	total := width - 10
	columns := []table.Column{
		{Title: "AGE", Width: total * 5 / 100},
		{Title: "TYPE", Width: total * 7 / 100},
		{Title: "REASON", Width: total * 13 / 100},
		{Title: "OBJECT", Width: total * 20 / 100},
		{Title: "NAMESPACE", Width: total * 12 / 100},
		{Title: "COUNT", Width: total * 5 / 100},
		{Title: "MESSAGE", Width: total * 35 / 100},
	}
	m.table.SetColumns(columns)
}

func (m *EventsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case EventsMsg:
		m.events = msg.Events
		m.loading = false
		if m.showAllNamespaces {
			m.eventsAllNS = msg.Events
		} else {
			m.eventsSingleNS = msg.Events
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case EventsAllNSMsg:
		m.eventsAllNS = msg.Events
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *EventsModel) View() string {
	if m.loading {
		return "Loading..."
	}

	row := m.table.SelectedRow()
	if len(row) > 1 && row[1] == "Warning" {
		m.table.SetStyles(eventTableStylesWarning())
	} else {
		m.table.SetStyles(eventTableStylesNormal())
	}

	return m.table.View()
}

func (m *EventsModel) Count() int {
	return m.filteredCount
}

func (m *EventsModel) Refresh() tea.Cmd {
	return m.fetchEvents
}

func (m *EventsModel) fetchEvents() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var events []corev1.Event
	var err error

	if m.showAllNamespaces {
		events, err = m.client.ListAllEvents(ctx)
	} else {
		events, err = m.client.ListEvents(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	sortEvents(events)
	return EventsMsg{Events: events}
}

func (m *EventsModel) fetchEventsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := m.client.ListAllEvents(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	sortEvents(events)
	return EventsAllNSMsg{Events: events}
}

func (m *EventsModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, ev := range m.events {
		objRef := fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name)
		if q != "" && !strings.Contains(strings.ToLower(ev.InvolvedObject.Name), q) &&
			!strings.Contains(strings.ToLower(ev.Reason), q) {
			continue
		}
		age := eventAge(ev)
		msg := ev.Message
		rows = append(rows, table.Row{
			age,
			ev.Type,
			ev.Reason,
			objRef,
			ev.Namespace,
			fmt.Sprintf("%d", ev.Count),
			msg,
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

func sortEvents(events []corev1.Event) {
	sort.Slice(events, func(i, j int) bool {
		ti := events[i].LastTimestamp.Time
		tj := events[j].LastTimestamp.Time
		return ti.After(tj)
	})
}

func eventAge(ev corev1.Event) string {
	t := ev.LastTimestamp.Time
	if t.IsZero() {
		t = ev.FirstTimestamp.Time
	}
	if t.IsZero() {
		return "?"
	}
	return formatAge(t)
}

type EventsMsg struct {
	Events []corev1.Event
}

type EventsAllNSMsg struct {
	Events []corev1.Event
}
