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

// PodsModel represents the pods view
type PodsModel struct {
	client            *client.Client
	table             table.Model
	pods              []corev1.Pod
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	// Cache for both modes
	podsAllNS    []corev1.Pod
	podsSingleNS []corev1.Pod
	cacheNS      string // namespace for single NS cache
}

// Table style functions for conditional formatting
func podTableStylesNormal() table.Styles {
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
		Background(lipgloss.Color("#005F87")). // Blue - Running
		Bold(true)
	return s
}

func podTableStylesWarning() table.Styles {
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
		Background(lipgloss.Color("#FFAF00")). // Yellow/Orange - Pending/Other
		Bold(true)
	return s
}

func podTableStylesError() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		MarginTop(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(ColorAccent)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#AF0000")). // Red - Error
		Bold(true)
	return s
}

// NewPodsModel creates a new pods model
func NewPodsModel(c *client.Client) *PodsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "READY", Width: 8},
		{Title: "STATUS", Width: 15},
		{Title: "RESTARTS", Width: 10},
		{Title: "AGE", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(podTableStylesNormal())

	return &PodsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *PodsModel) Init() tea.Cmd {
	return m.fetchPods
}

// InitAllNamespaces preloads all namespaces data for faster switching
func (m *PodsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchPodsAllNS
}

func (m *PodsModel) fetchPodsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pods, err := m.client.ListAllPods(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return PodsAllNSMsg{Pods: pods}
}

// PodsAllNSMsg is sent when all namespaces pods are loaded
type PodsAllNSMsg struct {
	Pods []corev1.Pod
}

// SetShowAllNamespaces sets the all namespaces flag and uses cache if available
func (m *PodsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	// Use cached data if available for instant switching
	if showAll && len(m.podsAllNS) > 0 {
		m.pods = m.podsAllNS
		m.updateTable()
	} else if !showAll && len(m.podsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.pods = m.podsSingleNS
		m.updateTable()
	}
}

// SetSize sets the view size
func (m *PodsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	// Adjust column widths
	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 15 / 100},
		{Title: "NAME", Width: totalWidth * 35 / 100},
		{Title: "READY", Width: totalWidth * 10 / 100},
		{Title: "STATUS", Width: totalWidth * 15 / 100},
		{Title: "RESTARTS", Width: totalWidth * 10 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *PodsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case PodsMsg:
		m.pods = msg.Pods
		m.loading = false
		// Update cache based on current mode
		if m.showAllNamespaces {
			m.podsAllNS = msg.Pods
		} else {
			m.podsSingleNS = msg.Pods
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case PodsAllNSMsg:
		// Cache all namespaces data (preload)
		m.podsAllNS = msg.Pods
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *PodsModel) View() string {
	if m.loading {
		return "Loading..."
	}

	// Apply conditional styling based on selected pod's status
	row := m.table.SelectedRow()
	if len(row) > 3 {
		status := row[3] // STATUS column
		switch {
		case status == "Running":
			m.table.SetStyles(podTableStylesNormal())
		case isErrorStatus(status):
			m.table.SetStyles(podTableStylesError())
		default:
			m.table.SetStyles(podTableStylesWarning())
		}
	}

	return m.table.View()
}

// isErrorStatus returns true if the status indicates an error
func isErrorStatus(status string) bool {
	errorStatuses := []string{
		"Error", "Failed", "CrashLoopBackOff", "ImagePullBackOff",
		"ErrImagePull", "OOMKilled", "CreateContainerError",
		"InvalidImageName", "CreateContainerConfigError",
	}
	for _, s := range errorStatuses {
		if status == s {
			return true
		}
	}
	return false
}

// Refresh fetches fresh data
func (m *PodsModel) Refresh() tea.Cmd {
	return m.fetchPods
}

// SetFilter sets the name filter and re-renders the table
func (m *PodsModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

// Count returns the number of displayed pods
func (m *PodsModel) Count() int {
	return m.filteredCount
}

// GetSelectedPod returns the currently selected pod
func (m *PodsModel) GetSelectedPod() *corev1.Pod {
	row := m.table.SelectedRow()
	if row == nil || len(m.pods) == 0 {
		return nil
	}

	// Find pod by namespace and name
	for i := range m.pods {
		if m.pods[i].Namespace == row[0] && m.pods[i].Name == row[1] {
			return &m.pods[i]
		}
	}
	return nil
}

func (m *PodsModel) fetchPods() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pods []corev1.Pod
	var err error

	if m.showAllNamespaces {
		pods, err = m.client.ListAllPods(ctx)
	} else {
		pods, err = m.client.ListPods(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return PodsMsg{Pods: pods}
}

func (m *PodsModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, pod := range m.pods {
		if q != "" && !strings.Contains(strings.ToLower(pod.Name), q) {
			continue
		}
		ready, total := getPodReadyCount(pod)
		status := getPodStatus(pod)
		restarts := getPodRestarts(pod)
		age := formatAge(pod.CreationTimestamp.Time)
		rows = append(rows, table.Row{
			pod.Namespace,
			pod.Name,
			fmt.Sprintf("%d/%d", ready, total),
			status,
			fmt.Sprintf("%d", restarts),
			age,
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// Message types
type PodsMsg struct {
	Pods []corev1.Pod
}

// Helper functions
func getPodStatus(pod corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}

	return string(pod.Status.Phase)
}

func getPodReadyCount(pod corev1.Pod) (int, int) {
	total := len(pod.Spec.Containers)
	ready := 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return ready, total
}

func getPodRestarts(pod corev1.Pod) int {
	restarts := 0
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += int(cs.RestartCount)
	}
	return restarts
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func formatLabels(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}
