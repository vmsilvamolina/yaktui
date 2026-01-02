package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/victor/yaktui/internal/client"
)

// Panel represents which panel is focused
type Panel int

const (
	SidebarPanel Panel = iota
	ContentPanel
)

// ResourceType represents the type of Kubernetes resource
type ResourceType int

const (
	ResourcePods ResourceType = iota
	ResourceDeployments
	ResourceServices
	ResourceConfigMaps
	ResourceSecrets
	ResourceNamespaces
)

func (r ResourceType) String() string {
	return [...]string{
		"Pods",
		"Deployments",
		"Services",
		"ConfigMaps",
		"Secrets",
		"Namespaces",
	}[r]
}

var resourceTypes = []ResourceType{
	ResourcePods,
	ResourceDeployments,
	ResourceServices,
	ResourceConfigMaps,
	ResourceSecrets,
	ResourceNamespaces,
}

// View represents the current view state
type View int

const (
	ViewList View = iota
	ViewRelations
	ViewLogs
	ViewDescribe
)

// ConnectionState represents the connection status
type ConnectionState int

const (
	Connecting ConnectionState = iota
	Connected
	ConnectionFailed
)

// Model is the main application model
type Model struct {
	client *client.Client
	keys   KeyMap
	width  int
	height int
	ready  bool

	// Connection state
	connectionState ConnectionState
	connectionError error
	kubeconfig      string

	// Panels
	focusedPanel Panel

	// Sidebar state
	sidebarItems    []ResourceType
	sidebarSelected int

	// Content state
	currentResource ResourceType
	currentView     View

	// Global namespace filter
	showAllNamespaces bool

	// Resource views
	podsView        *PodsModel
	deploymentsView *DeploymentsModel
	servicesView    *ServicesModel
	configmapsView  *ConfigMapsModel
	secretsView     *SecretsModel
	namespacesView  *NamespacesModel

	// Special views
	relationsView *RelationsModel
	logsView      *LogsModel

	// Status
	statusMessage string
	err           error
}

// NewModel creates a new application model
func NewModel(c *client.Client, kubeconfig string) Model {
	return Model{
		client:          c,
		keys:            DefaultKeyMap,
		kubeconfig:      kubeconfig,
		connectionState: Connecting,
		focusedPanel:    ContentPanel,
		sidebarItems:    resourceTypes,
		sidebarSelected: 0,
		currentResource: ResourcePods,
		currentView:     ViewList,
		podsView:        NewPodsModel(c),
		deploymentsView: NewDeploymentsModel(c),
		servicesView:    NewServicesModel(c),
		configmapsView:  NewConfigMapsModel(c),
		secretsView:     NewSecretsModel(c),
		namespacesView:  NewNamespacesModel(c),
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	// First check connection, then load resources
	return tea.Batch(
		m.checkConnection,
		tea.EnterAltScreen,
	)
}

// checkConnection verifies the connection to the cluster
func (m Model) checkConnection() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.client.CheckConnection(ctx); err != nil {
		return ConnectionResultMsg{Connected: false, Err: err}
	}
	return ConnectionResultMsg{Connected: true}
}

// loadAllResources loads all resources after successful connection
func (m Model) loadAllResources() tea.Cmd {
	return tea.Batch(
		m.podsView.Init(),
		m.podsView.InitAllNamespaces(),
		m.deploymentsView.Init(),
		m.deploymentsView.InitAllNamespaces(),
		m.servicesView.Init(),
		m.servicesView.InitAllNamespaces(),
		m.configmapsView.Init(),
		m.configmapsView.InitAllNamespaces(),
		m.secretsView.Init(),
		m.secretsView.InitAllNamespaces(),
		m.namespacesView.Init(),
	)
}

// ConnectionResultMsg is sent when connection check completes
type ConnectionResultMsg struct {
	Connected bool
	Err       error
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keys
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Retry connection on 'r' when connection failed
		if m.connectionState == ConnectionFailed && msg.String() == "r" {
			m.connectionState = Connecting
			m.connectionError = nil
			return m, m.checkConnection
		}

		// Don't process other keys if not connected
		if m.connectionState != Connected {
			return m, nil
		}

		// Global toggle for all namespaces
		if key.Matches(msg, m.keys.AllNS) && m.currentView == ViewList {
			m.showAllNamespaces = !m.showAllNamespaces
			// Propagate to all views - they will use cache if available
			m.podsView.SetShowAllNamespaces(m.showAllNamespaces)
			m.deploymentsView.SetShowAllNamespaces(m.showAllNamespaces)
			m.servicesView.SetShowAllNamespaces(m.showAllNamespaces)
			m.configmapsView.SetShowAllNamespaces(m.showAllNamespaces)
			m.secretsView.SetShowAllNamespaces(m.showAllNamespaces)
			// No need to fetch again - SetShowAllNamespaces uses cached data
			return m, nil
		}

		// Handle based on current view
		if m.currentView == ViewLogs && m.logsView != nil {
			if key.Matches(msg, m.keys.Back) {
				m.currentView = ViewList
				m.logsView = nil
				return m, nil
			}
			newLogsView, cmd := m.logsView.Update(msg)
			m.logsView = newLogsView.(*LogsModel)
			return m, cmd
		}

		if m.currentView == ViewRelations && m.relationsView != nil {
			if key.Matches(msg, m.keys.Back) {
				m.currentView = ViewList
				m.relationsView = nil
				return m, nil
			}
			// Handle logs from relations view
			if key.Matches(msg, m.keys.Logs) || key.Matches(msg, m.keys.Enter) {
				if pod := m.relationsView.GetSelectedPod(); pod != nil {
					m.logsView = NewLogsModel(m.client, pod)
					m.logsView.SetSize(m.width-27, m.height-6)
					m.currentView = ViewLogs
					return m, m.logsView.Init()
				}
			}
			newRelView, cmd := m.relationsView.Update(msg)
			m.relationsView = newRelView.(*RelationsModel)
			return m, cmd
		}

		// Panel switching
		if key.Matches(msg, m.keys.Tab) {
			if m.focusedPanel == SidebarPanel {
				m.focusedPanel = ContentPanel
			} else {
				m.focusedPanel = SidebarPanel
			}
			return m, nil
		}

		if m.focusedPanel == SidebarPanel {
			return m.updateSidebar(msg)
		}

		return m.updateContent(msg)

	case ConnectionResultMsg:
		if msg.Connected {
			m.connectionState = Connected
			return m, m.loadAllResources()
		} else {
			m.connectionState = ConnectionFailed
			m.connectionError = msg.Err
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update all view sizes (account for sidebar + borders + horizontal margins)
		contentWidth := m.width - 27
		contentHeight := m.height - 6

		if m.podsView != nil {
			m.podsView.SetSize(contentWidth, contentHeight)
		}
		if m.deploymentsView != nil {
			m.deploymentsView.SetSize(contentWidth, contentHeight)
		}
		if m.servicesView != nil {
			m.servicesView.SetSize(contentWidth, contentHeight)
		}
		if m.configmapsView != nil {
			m.configmapsView.SetSize(contentWidth, contentHeight)
		}
		if m.secretsView != nil {
			m.secretsView.SetSize(contentWidth, contentHeight)
		}
		if m.namespacesView != nil {
			m.namespacesView.SetSize(contentWidth, contentHeight)
		}
		if m.relationsView != nil {
			m.relationsView.SetSize(contentWidth, contentHeight)
		}
		if m.logsView != nil {
			m.logsView.SetSize(contentWidth, contentHeight)
		}

		return m, nil

	case RelationsMsg:
		// Forward to relations view
		if m.relationsView != nil {
			newView, cmd := m.relationsView.Update(msg)
			m.relationsView = newView.(*RelationsModel)
			return m, cmd
		}
		return m, nil

	case LogsMsg, LogsTickMsg:
		// Forward to logs view
		if m.logsView != nil {
			newView, cmd := m.logsView.Update(msg)
			m.logsView = newView.(*LogsModel)
			return m, cmd
		}
		return m, nil

	case TickMsg:
		// Refresh current view data
		return m.refreshCurrentView()

	case ErrorMsg:
		m.err = msg.Err
		m.statusMessage = "Error: " + msg.Err.Error()
		return m, nil
	}

	// Handle resource-specific messages
	switch msg.(type) {
	case PodsMsg, PodsAllNSMsg:
		if m.podsView != nil {
			newView, cmd := m.podsView.Update(msg)
			m.podsView = newView.(*PodsModel)
			return m, cmd
		}
	case DeploymentsMsg, DeploymentsAllNSMsg:
		if m.deploymentsView != nil {
			newView, cmd := m.deploymentsView.Update(msg)
			m.deploymentsView = newView.(*DeploymentsModel)
			return m, cmd
		}
	case ServicesMsg, ServicesAllNSMsg:
		if m.servicesView != nil {
			newView, cmd := m.servicesView.Update(msg)
			m.servicesView = newView.(*ServicesModel)
			return m, cmd
		}
	case ConfigMapsMsg, ConfigMapsAllNSMsg:
		if m.configmapsView != nil {
			newView, cmd := m.configmapsView.Update(msg)
			m.configmapsView = newView.(*ConfigMapsModel)
			return m, cmd
		}
	case SecretsMsg, SecretsAllNSMsg:
		if m.secretsView != nil {
			newView, cmd := m.secretsView.Update(msg)
			m.secretsView = newView.(*SecretsModel)
			return m, cmd
		}
	case NamespacesMsg:
		if m.namespacesView != nil {
			newView, cmd := m.namespacesView.Update(msg)
			m.namespacesView = newView.(*NamespacesModel)
			return m, cmd
		}
	}

	// Update current resource view
	return m.updateCurrentView(msg)
}

func (m Model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.sidebarSelected > 0 {
			m.sidebarSelected--
			// Change resource but keep focus on sidebar
			m.currentResource = m.sidebarItems[m.sidebarSelected]
			m.focusedPanel = SidebarPanel // Explicitly keep focus on sidebar
			return m, m.initCurrentView()
		}
	case key.Matches(msg, m.keys.Down):
		if m.sidebarSelected < len(m.sidebarItems)-1 {
			m.sidebarSelected++
			// Change resource but keep focus on sidebar
			m.currentResource = m.sidebarItems[m.sidebarSelected]
			m.focusedPanel = SidebarPanel // Explicitly keep focus on sidebar
			return m, m.initCurrentView()
		}
	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Right):
		// Only change focus when explicitly pressing Enter/Right
		m.currentResource = m.sidebarItems[m.sidebarSelected]
		m.focusedPanel = ContentPanel
		return m, m.initCurrentView()
	}
	return m, nil
}

func (m Model) updateContent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Navigate back to sidebar
	if key.Matches(msg, m.keys.Left) || key.Matches(msg, m.keys.Back) {
		m.focusedPanel = SidebarPanel
		return m, nil
	}

	// Resource-specific keys
	switch m.currentResource {
	case ResourcePods:
		if key.Matches(msg, m.keys.Enter) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.relationsView = NewRelationsModel(m.client, pod)
				m.relationsView.SetSize(m.width-27, m.height-6)
				m.currentView = ViewRelations
				return m, m.relationsView.Init()
			}
		}
		if key.Matches(msg, m.keys.Logs) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.logsView = NewLogsModel(m.client, pod)
				m.logsView.SetSize(m.width-27, m.height-6)
				m.currentView = ViewLogs
				return m, m.logsView.Init()
			}
		}
		newView, cmd := m.podsView.Update(msg)
		m.podsView = newView.(*PodsModel)
		return m, cmd

	case ResourceDeployments:
		newView, cmd := m.deploymentsView.Update(msg)
		m.deploymentsView = newView.(*DeploymentsModel)
		return m, cmd

	case ResourceServices:
		newView, cmd := m.servicesView.Update(msg)
		m.servicesView = newView.(*ServicesModel)
		return m, cmd

	case ResourceConfigMaps:
		newView, cmd := m.configmapsView.Update(msg)
		m.configmapsView = newView.(*ConfigMapsModel)
		return m, cmd

	case ResourceSecrets:
		newView, cmd := m.secretsView.Update(msg)
		m.secretsView = newView.(*SecretsModel)
		return m, cmd

	case ResourceNamespaces:
		if key.Matches(msg, m.keys.Enter) {
			if ns := m.namespacesView.GetSelectedNamespace(); ns != "" {
				m.client.SetNamespace(ns)
				m.statusMessage = "Switched to namespace: " + ns
				// Also switch to single namespace mode
				m.showAllNamespaces = false
				m.podsView.SetShowAllNamespaces(false)
				m.deploymentsView.SetShowAllNamespaces(false)
				m.servicesView.SetShowAllNamespaces(false)
				m.configmapsView.SetShowAllNamespaces(false)
				m.secretsView.SetShowAllNamespaces(false)
				// Refresh all views
				return m, tea.Batch(
					m.podsView.Init(),
					m.deploymentsView.Init(),
					m.servicesView.Init(),
					m.configmapsView.Init(),
					m.secretsView.Init(),
				)
			}
		}
		newView, cmd := m.namespacesView.Update(msg)
		m.namespacesView = newView.(*NamespacesModel)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentResource {
	case ResourcePods:
		newView, cmd := m.podsView.Update(msg)
		m.podsView = newView.(*PodsModel)
		return m, cmd
	case ResourceDeployments:
		newView, cmd := m.deploymentsView.Update(msg)
		m.deploymentsView = newView.(*DeploymentsModel)
		return m, cmd
	case ResourceServices:
		newView, cmd := m.servicesView.Update(msg)
		m.servicesView = newView.(*ServicesModel)
		return m, cmd
	case ResourceConfigMaps:
		newView, cmd := m.configmapsView.Update(msg)
		m.configmapsView = newView.(*ConfigMapsModel)
		return m, cmd
	case ResourceSecrets:
		newView, cmd := m.secretsView.Update(msg)
		m.secretsView = newView.(*SecretsModel)
		return m, cmd
	case ResourceNamespaces:
		newView, cmd := m.namespacesView.Update(msg)
		m.namespacesView = newView.(*NamespacesModel)
		return m, cmd
	}
	return m, nil
}

func (m Model) initCurrentView() tea.Cmd {
	switch m.currentResource {
	case ResourcePods:
		return m.podsView.Init()
	case ResourceDeployments:
		return m.deploymentsView.Init()
	case ResourceServices:
		return m.servicesView.Init()
	case ResourceConfigMaps:
		return m.configmapsView.Init()
	case ResourceSecrets:
		return m.secretsView.Init()
	case ResourceNamespaces:
		return m.namespacesView.Init()
	}
	return nil
}

func (m Model) refreshCurrentView() (tea.Model, tea.Cmd) {
	switch m.currentResource {
	case ResourcePods:
		return m, m.podsView.Refresh()
	case ResourceDeployments:
		return m, m.deploymentsView.Refresh()
	case ResourceServices:
		return m, m.servicesView.Refresh()
	case ResourceConfigMaps:
		return m, m.configmapsView.Refresh()
	case ResourceSecrets:
		return m, m.secretsView.Refresh()
	case ResourceNamespaces:
		return m, m.namespacesView.Refresh()
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header with padding
	header := lipgloss.NewStyle().
		Padding(1, 0).
		Render(LogoStyle.Render("  YAKTUI - Yet Another Kubernetes TUI"))

	// Show connection status if not connected
	if m.connectionState == Connecting {
		return m.renderConnectingView(header)
	}
	if m.connectionState == ConnectionFailed {
		return m.renderConnectionErrorView(header)
	}

	// Sidebar
	sidebar := m.renderSidebar()

	// Content
	content := m.renderContent()

	// Status bar
	status := m.renderStatusBar()

	// Layout with horizontal padding
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	// Add horizontal margin to match the gap between panels
	paddedContent := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		paddedContent,
		status,
	)
}

func (m Model) renderSidebar() string {
	var items string
	for i, item := range m.sidebarItems {
		style := SidebarItemStyle
		prefix := "  "
		if i == m.sidebarSelected {
			style = SidebarSelectedStyle
			prefix = "▸ "
		}
		items += style.Render(prefix+item.String()) + "\n"
	}

	sidebarStyle := SidebarStyle
	if m.focusedPanel == SidebarPanel {
		sidebarStyle = SidebarFocusedStyle
	}

	title := TitleStyle.Render("Resources")
	sidebar := title + "\n" + items

	sidebarHeight := m.height - 6
	if sidebarHeight < 5 {
		sidebarHeight = 5
	}

	return sidebarStyle.
		Width(20).
		Height(sidebarHeight).
		Render(sidebar)
}

func (m Model) renderContent() string {
	var content string
	var title string
	var count int

	switch m.currentView {
	case ViewLogs:
		if m.logsView != nil {
			content = m.logsView.View()
			title = "Logs: " + m.logsView.GetLogTitle()
		}
	case ViewRelations:
		if m.relationsView != nil {
			content = m.relationsView.View()
			title = "Relations: " + m.relationsView.GetPodName()
		}
	default:
		switch m.currentResource {
		case ResourcePods:
			content = m.podsView.View()
			count = m.podsView.Count()
		case ResourceDeployments:
			content = m.deploymentsView.View()
			count = m.deploymentsView.Count()
		case ResourceServices:
			content = m.servicesView.View()
			count = m.servicesView.Count()
		case ResourceConfigMaps:
			content = m.configmapsView.View()
			count = m.configmapsView.Count()
		case ResourceSecrets:
			content = m.secretsView.View()
			count = m.secretsView.Count()
		case ResourceNamespaces:
			content = m.namespacesView.View()
			count = m.namespacesView.Count()
		}
		// Build title with namespace info and count
		nsInfo := ""
		if m.showAllNamespaces {
			nsInfo = "(all namespaces)"
		} else {
			nsInfo = "[" + m.client.Namespace() + "]"
		}
		title = fmt.Sprintf("%s %s (%d)", m.currentResource.String(), nsInfo, count)
	}

	// Account for sidebar (20) + borders (4) + horizontal margins (2)
	contentWidth := m.width - 24
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Account for header (3 with padding) + status bar (1) + borders
	contentHeight := m.height - 4
	if contentHeight < 5 {
		contentHeight = 5
	}

	focused := m.focusedPanel == ContentPanel
	return RenderPanelWithTitle(title, content, contentWidth, contentHeight, focused)
}

func (m Model) renderConnectingView(header string) string {
	content := lipgloss.NewStyle().
		Width(m.width - 4).
		Height(m.height - 6).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(
			lipgloss.NewStyle().Foreground(ColorPrimary).Render("⟳ Connecting to cluster...") + "\n\n" +
				lipgloss.NewStyle().Foreground(ColorMuted).Render("kubeconfig: "+m.kubeconfig),
		)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (m Model) renderConnectionErrorView(header string) string {
	errorStyle := lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	errorMsg := errorStyle.Render("✗ Failed to connect to cluster") + "\n\n"
	// if m.connectionError != nil {
	// 	errorMsg += mutedStyle.Render(m.connectionError.Error()) + "\n\n"
	// }
	errorMsg += accentStyle.Render("Please check:") + "\n" +
		mutedStyle.Render("  • kubeconfig file exists: "+m.kubeconfig) + "\n" +
		mutedStyle.Render("  • cluster is running and accessible") + "\n" +
		mutedStyle.Render("  • credentials are valid") + "\n\n" +
		lipgloss.NewStyle().Foreground(ColorWarning).Render("Press 'r' to retry or 'ctrl+c' to quit")

	content := lipgloss.NewStyle().
		Width(m.width - 4).
		Height(m.height - 6).
		Padding(2).
		Render(errorMsg)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (m Model) renderStatusBar() string {
	var help string

	switch m.currentView {
	case ViewLogs:
		help = StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" back") + "  " +
			StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
	case ViewRelations:
		help = StatusKeyStyle.Render("↑↓") + StatusDescStyle.Render(" navigate") + "  " +
			StatusKeyStyle.Render("enter/l") + StatusDescStyle.Render(" logs") + "  " +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" back") + "  " +
			StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
	default:
		if m.currentResource == ResourcePods {
			help = StatusKeyStyle.Render("tab") + StatusDescStyle.Render(" switch") + "  " +
				StatusKeyStyle.Render("enter") + StatusDescStyle.Render(" relations") + "  " +
				StatusKeyStyle.Render("l") + StatusDescStyle.Render(" logs") + "  " +
				StatusKeyStyle.Render("a") + StatusDescStyle.Render(" all ns") + "  " +
				StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
		} else {
			help = StatusKeyStyle.Render("tab") + StatusDescStyle.Render(" switch") + "  " +
				StatusKeyStyle.Render("a") + StatusDescStyle.Render(" all ns") + "  " +
				StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
		}
	}

	if m.statusMessage != "" {
		help = m.statusMessage + "  |  " + help
	}

	return StatusBarStyle.Width(m.width).Render(help)
}

// Message types
type TickMsg struct{}
type ErrorMsg struct{ Err error }
type AllNamespacesToggleMsg struct{}
