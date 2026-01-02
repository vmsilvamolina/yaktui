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

// SecretsModel represents the secrets view
type SecretsModel struct {
	client            *client.Client
	table             table.Model
	secrets           []corev1.Secret
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	// Cache for both modes
	secretsAllNS    []corev1.Secret
	secretsSingleNS []corev1.Secret
	cacheNS         string
}

// NewSecretsModel creates a new secrets model
func NewSecretsModel(c *client.Client) *SecretsModel {
	columns := []table.Column{
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "TYPE", Width: 30},
		{Title: "DATA", Width: 8},
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

	return &SecretsModel{
		client:  c,
		table:   t,
		loading: true,
	}
}

// Init initializes the model
func (m *SecretsModel) Init() tea.Cmd {
	return m.fetchSecrets
}

// InitAllNamespaces preloads all namespaces data
func (m *SecretsModel) InitAllNamespaces() tea.Cmd {
	return m.fetchSecretsAllNS
}

func (m *SecretsModel) fetchSecretsAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secrets, err := m.client.ListAllSecrets(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return SecretsAllNSMsg{Secrets: secrets}
}

type SecretsAllNSMsg struct {
	Secrets []corev1.Secret
}

// SetShowAllNamespaces sets the all namespaces flag and uses cache if available
func (m *SecretsModel) SetShowAllNamespaces(showAll bool) {
	m.showAllNamespaces = showAll
	if showAll && len(m.secretsAllNS) > 0 {
		m.secrets = m.secretsAllNS
		m.updateTable()
	} else if !showAll && len(m.secretsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.secrets = m.secretsSingleNS
		m.updateTable()
	}
}

// SetSize sets the view size
func (m *SecretsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "NAMESPACE", Width: totalWidth * 15 / 100},
		{Title: "NAME", Width: totalWidth * 30 / 100},
		{Title: "TYPE", Width: totalWidth * 30 / 100},
		{Title: "DATA", Width: totalWidth * 10 / 100},
		{Title: "AGE", Width: totalWidth * 10 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *SecretsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SecretsMsg:
		m.secrets = msg.Secrets
		m.loading = false
		if m.showAllNamespaces {
			m.secretsAllNS = msg.Secrets
		} else {
			m.secretsSingleNS = msg.Secrets
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case SecretsAllNSMsg:
		m.secretsAllNS = msg.Secrets
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *SecretsModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Count returns the number of secrets
func (m *SecretsModel) Count() int {
	return len(m.secrets)
}

// Refresh fetches fresh data
func (m *SecretsModel) Refresh() tea.Cmd {
	return m.fetchSecrets
}

func (m *SecretsModel) fetchSecrets() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var secrets []corev1.Secret
	var err error

	if m.showAllNamespaces {
		secrets, err = m.client.ListAllSecrets(ctx)
	} else {
		secrets, err = m.client.ListSecrets(ctx)
	}

	if err != nil {
		return ErrorMsg{Err: err}
	}

	return SecretsMsg{Secrets: secrets}
}

func (m *SecretsModel) updateTable() {
	rows := make([]table.Row, len(m.secrets))

	for i, secret := range m.secrets {
		rows[i] = table.Row{
			secret.Namespace,
			secret.Name,
			string(secret.Type),
			fmt.Sprintf("%d", len(secret.Data)),
			formatAge(secret.CreationTimestamp.Time),
		}
	}

	m.table.SetRows(rows)
}

// Message types
type SecretsMsg struct {
	Secrets []corev1.Secret
}
