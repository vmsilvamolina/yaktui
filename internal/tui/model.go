package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/client"
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
	ResourceNodes
	ResourceStatefulSets
	ResourceDaemonSets
	ResourceEvents
)

func (r ResourceType) String() string {
	return [...]string{
		"Pods",
		"Deployments",
		"Services",
		"ConfigMaps",
		"Secrets",
		"Namespaces",
		"Nodes",
		"StatefulSets",
		"DaemonSets",
		"Events",
	}[r]
}

var resourceTypes = []ResourceType{
	ResourcePods,
	ResourceDeployments,
	ResourceStatefulSets,
	ResourceDaemonSets,
	ResourceServices,
	ResourceConfigMaps,
	ResourceSecrets,
	ResourceNamespaces,
	ResourceNodes,
	ResourceEvents,
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
	clusterInfo     client.ClusterInfo

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
	nodesView        *NodesModel
	statefulSetsView *StatefulSetsModel
	daemonSetsView   *DaemonSetsModel

	eventsView       *EventsModel

	// Special views
	relationsView *RelationsModel
	logsView      *LogsModel
	describeView  *DescribeModel

	// Delete confirmation
	confirmDelete       bool
	confirmDeleteTarget string

	// Search/filter
	filterMode  bool
	filterQuery string

	// Command palette
	commandMode    bool
	commandInput   string
	commandMatches []commandMatch

	// Context switch
	contextMode     bool
	contexts        []string
	contextCurrent  string
	contextSelected int

	// Help overlay
	showHelp bool

	// Status
	statusMessage string
	err           error
}

type commandMatch struct {
	label    string
	resource ResourceType
}

var paletteAliases = []struct {
	alias    string
	resource ResourceType
}{
	{"pod", ResourcePods},
	{"pods", ResourcePods},
	{"deploy", ResourceDeployments},
	{"deployment", ResourceDeployments},
	{"deployments", ResourceDeployments},
	{"svc", ResourceServices},
	{"service", ResourceServices},
	{"services", ResourceServices},
	{"cm", ResourceConfigMaps},
	{"configmap", ResourceConfigMaps},
	{"configmaps", ResourceConfigMaps},
	{"secret", ResourceSecrets},
	{"secrets", ResourceSecrets},
	{"ns", ResourceNamespaces},
	{"namespace", ResourceNamespaces},
	{"namespaces", ResourceNamespaces},
	{"node", ResourceNodes},
	{"nodes", ResourceNodes},
	{"sts", ResourceStatefulSets},
	{"statefulset", ResourceStatefulSets},
	{"statefulsets", ResourceStatefulSets},
	{"ds", ResourceDaemonSets},
	{"daemonset", ResourceDaemonSets},
	{"daemonsets", ResourceDaemonSets},
	{"event", ResourceEvents},
	{"events", ResourceEvents},
}

func matchPalette(input string) []commandMatch {
	if input == "" {
		return nil
	}
	lower := strings.ToLower(input)
	seen := map[ResourceType]bool{}
	var matches []commandMatch
	for _, a := range paletteAliases {
		if strings.HasPrefix(a.alias, lower) && !seen[a.resource] {
			seen[a.resource] = true
			matches = append(matches, commandMatch{label: a.resource.String(), resource: a.resource})
		}
	}
	return matches
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
		nodesView:        NewNodesModel(c),
		statefulSetsView: NewStatefulSetsModel(c),
		daemonSetsView:   NewDaemonSetsModel(c),
		eventsView:       NewEventsModel(c),
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

func tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
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

func (m Model) fetchServerVersion() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, err := m.client.FetchServerVersion(ctx)
	if err != nil {
		return ClusterInfoMsg{}
	}
	return ClusterInfoMsg{Version: version}
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
		m.nodesView.Init(),
		m.statefulSetsView.Init(),
		m.statefulSetsView.InitAllNamespaces(),
		m.daemonSetsView.Init(),
		m.daemonSetsView.InitAllNamespaces(),
		m.eventsView.Init(),
		m.eventsView.InitAllNamespaces(),
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
		// Confirmation modal intercepts all keys
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				m.confirmDelete = false
				return m, m.deletePod(m.confirmDeleteTarget)
			case "n", "N", "esc":
				m.confirmDelete = false
				m.confirmDeleteTarget = ""
			}
			return m, nil
		}

		// Filter mode intercepts input
		if m.filterMode {
			switch msg.String() {
			case "esc":
				m.filterMode = false
				m.filterQuery = ""
				m.setFilterOnCurrentView()
			case "enter":
				m.filterMode = false
			case "backspace", "ctrl+h":
				if len(m.filterQuery) > 0 {
					runes := []rune(m.filterQuery)
					m.filterQuery = string(runes[:len(runes)-1])
					m.setFilterOnCurrentView()
				}
			default:
				if key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.Down) {
					return m.updateCurrentView(msg)
				}
				if len(msg.Runes) > 0 {
					m.filterQuery += string(msg.Runes)
					m.setFilterOnCurrentView()
				}
			}
			return m, nil
		}

		// Help overlay — ? toggles, esc closes, ctrl+c still quits
		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			if key.Matches(msg, m.keys.Back) {
				m.showHelp = false
			} else if key.Matches(msg, m.keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		// Command palette intercepts input
		if m.commandMode {
			switch msg.String() {
			case "esc":
				m.commandMode = false
				m.commandInput = ""
				m.commandMatches = nil
			case "enter":
				if len(m.commandMatches) > 0 {
					resource := m.commandMatches[0].resource
					m.commandMode = false
					m.commandInput = ""
					m.commandMatches = nil
					return m.navigateToResource(resource)
				}
				m.commandMode = false
				m.commandInput = ""
				m.commandMatches = nil
			case "backspace", "ctrl+h":
				if len(m.commandInput) > 0 {
					runes := []rune(m.commandInput)
					m.commandInput = string(runes[:len(runes)-1])
					m.commandMatches = matchPalette(m.commandInput)
				}
			default:
				if len(msg.Runes) > 0 {
					m.commandInput += string(msg.Runes)
					m.commandMatches = matchPalette(m.commandInput)
				}
			}
			return m, nil
		}

		// Context switch modal intercepts navigation
		if m.contextMode {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Back):
				m.contextMode = false
				m.contexts = nil
			case key.Matches(msg, m.keys.Up):
				if m.contextSelected > 0 {
					m.contextSelected--
				}
			case key.Matches(msg, m.keys.Down):
				if m.contextSelected < len(m.contexts)-1 {
					m.contextSelected++
				}
			case key.Matches(msg, m.keys.Enter):
				if m.contextSelected < len(m.contexts) {
					chosen := m.contexts[m.contextSelected]
					return m, m.switchContext(chosen)
				}
			}
			return m, nil
		}

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
			m.statefulSetsView.SetShowAllNamespaces(m.showAllNamespaces)
			m.daemonSetsView.SetShowAllNamespaces(m.showAllNamespaces)
			m.eventsView.SetShowAllNamespaces(m.showAllNamespaces)
			// No need to fetch again - SetShowAllNamespaces uses cached data
			return m, nil
		}

		// Context switch
		if key.Matches(msg, m.keys.ContextSwitch) && m.currentView == ViewList {
			return m, m.fetchContexts
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

		if m.currentView == ViewDescribe && m.describeView != nil {
			if key.Matches(msg, m.keys.Back) {
				m.currentView = ViewList
				m.describeView = nil
				return m, nil
			}
			newView, cmd := m.describeView.Update(msg)
			m.describeView = newView.(*DescribeModel)
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
					m.logsView.SetSize(m.width-27, m.height-7)
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
			m.clusterInfo = m.client.GetClusterInfo()
			return m, tea.Batch(m.loadAllResources(), tickCmd(), m.fetchServerVersion)
		} else {
			m.connectionState = ConnectionFailed
			m.connectionError = msg.Err
			return m, nil
		}

	case ClusterInfoMsg:
		m.clusterInfo.Version = msg.Version
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update all view sizes (account for sidebar + borders + horizontal margins)
		contentWidth := m.width - 27
		contentHeight := m.height - 7

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
		if m.nodesView != nil {
			m.nodesView.SetSize(contentWidth, contentHeight)
		}
		if m.statefulSetsView != nil {
			m.statefulSetsView.SetSize(contentWidth, contentHeight)
		}
		if m.daemonSetsView != nil {
			m.daemonSetsView.SetSize(contentWidth, contentHeight)
		}
		if m.eventsView != nil {
			m.eventsView.SetSize(contentWidth, contentHeight)
		}
		if m.relationsView != nil {
			m.relationsView.SetSize(contentWidth, contentHeight)
		}
		if m.logsView != nil {
			m.logsView.SetSize(contentWidth, contentHeight)
		}
		if m.describeView != nil {
			m.describeView.SetSize(contentWidth, contentHeight)
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

	case LogsMsg, LogsTickMsg, LogsErrorMsg:
		// Forward to logs view
		if m.logsView != nil {
			newView, cmd := m.logsView.Update(msg)
			m.logsView = newView.(*LogsModel)
			return m, cmd
		}
		return m, nil

	case TickMsg:
		newModel, cmd := m.refreshCurrentView()
		return newModel, tea.Batch(cmd, tickCmd())

	case ErrorMsg:
		m.err = msg.Err
		m.statusMessage = "Error: " + msg.Err.Error()
		return m, nil

	case ExecFinishedMsg:
		if msg.Err != nil {
			m.statusMessage = "exec: " + msg.Err.Error()
		} else {
			m.statusMessage = "exec session ended"
		}
		return m, nil

	case DeletePodResultMsg:
		if msg.Err != nil {
			m.statusMessage = "error deleting " + msg.Name + ": " + msg.Err.Error()
		} else {
			m.statusMessage = "deleted pod: " + msg.Name
			return m, m.podsView.Refresh()
		}
		return m, nil

	case ContextsMsg:
		if msg.Err != nil {
			m.statusMessage = "error listing contexts: " + msg.Err.Error()
			return m, nil
		}
		m.contexts = msg.Contexts
		m.contextCurrent = m.clusterInfo.Context
		m.contextSelected = 0
		for i, ctx := range msg.Contexts {
			if ctx == m.contextCurrent {
				m.contextSelected = i
				break
			}
		}
		m.contextMode = true
		return m, nil

	case ContextSwitchedMsg:
		if msg.Err != nil {
			m.contextMode = false
			m.statusMessage = "context switch failed: " + msg.Err.Error()
			return m, nil
		}
		m.client = msg.NewClient
		m.clusterInfo = msg.NewClient.GetClusterInfo()
		m.contextMode = false
		m.contexts = nil
		m.showAllNamespaces = false
		m.podsView = NewPodsModel(m.client)
		m.deploymentsView = NewDeploymentsModel(m.client)
		m.servicesView = NewServicesModel(m.client)
		m.configmapsView = NewConfigMapsModel(m.client)
		m.secretsView = NewSecretsModel(m.client)
		m.namespacesView = NewNamespacesModel(m.client)
		m.nodesView = NewNodesModel(m.client)
		m.statefulSetsView = NewStatefulSetsModel(m.client)
		m.daemonSetsView = NewDaemonSetsModel(m.client)
		m.eventsView = NewEventsModel(m.client)
		contentWidth := m.width - 27
		contentHeight := m.height - 7
		m.podsView.SetSize(contentWidth, contentHeight)
		m.deploymentsView.SetSize(contentWidth, contentHeight)
		m.servicesView.SetSize(contentWidth, contentHeight)
		m.configmapsView.SetSize(contentWidth, contentHeight)
		m.secretsView.SetSize(contentWidth, contentHeight)
		m.namespacesView.SetSize(contentWidth, contentHeight)
		m.nodesView.SetSize(contentWidth, contentHeight)
		m.statefulSetsView.SetSize(contentWidth, contentHeight)
		m.daemonSetsView.SetSize(contentWidth, contentHeight)
		m.eventsView.SetSize(contentWidth, contentHeight)
		m.statusMessage = "switched to context: " + m.clusterInfo.Context
		return m, tea.Batch(m.loadAllResources(), m.fetchServerVersion)
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
	case NodesMsg:
		if m.nodesView != nil {
			newView, cmd := m.nodesView.Update(msg)
			m.nodesView = newView.(*NodesModel)
			return m, cmd
		}
	case StatefulSetsMsg, StatefulSetsAllNSMsg:
		if m.statefulSetsView != nil {
			newView, cmd := m.statefulSetsView.Update(msg)
			m.statefulSetsView = newView.(*StatefulSetsModel)
			return m, cmd
		}
	case DaemonSetsMsg, DaemonSetsAllNSMsg:
		if m.daemonSetsView != nil {
			newView, cmd := m.daemonSetsView.Update(msg)
			m.daemonSetsView = newView.(*DaemonSetsModel)
			return m, cmd
		}
	case EventsMsg, EventsAllNSMsg:
		if m.eventsView != nil {
			newView, cmd := m.eventsView.Update(msg)
			m.eventsView = newView.(*EventsModel)
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
			m.currentResource = m.sidebarItems[m.sidebarSelected]
			m.focusedPanel = SidebarPanel
			m.filterMode = false
			m.filterQuery = ""
			return m, m.initCurrentView()
		}
	case key.Matches(msg, m.keys.Down):
		if m.sidebarSelected < len(m.sidebarItems)-1 {
			m.sidebarSelected++
			m.currentResource = m.sidebarItems[m.sidebarSelected]
			m.focusedPanel = SidebarPanel
			m.filterMode = false
			m.filterQuery = ""
			return m, m.initCurrentView()
		}
	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Right):
		m.currentResource = m.sidebarItems[m.sidebarSelected]
		m.focusedPanel = ContentPanel
		m.filterMode = false
		m.filterQuery = ""
		return m, m.initCurrentView()
	}
	return m, nil
}

func (m Model) updateContent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Enter command palette mode
	if key.Matches(msg, m.keys.CommandPalette) && m.currentView == ViewList {
		m.commandMode = true
		m.commandInput = ""
		m.commandMatches = nil
		return m, nil
	}

	// Enter filter mode
	if key.Matches(msg, m.keys.Search) && m.currentView == ViewList {
		m.filterMode = true
		return m, nil
	}

	// Navigate back to sidebar (clears active filter first)
	if key.Matches(msg, m.keys.Left) || key.Matches(msg, m.keys.Back) {
		if m.filterQuery != "" {
			m.filterQuery = ""
			m.setFilterOnCurrentView()
			return m, nil
		}
		m.focusedPanel = SidebarPanel
		return m, nil
	}

	// Describe key — works for all resource types
	if key.Matches(msg, m.keys.Describe) {
		switch m.currentResource {
		case ResourcePods:
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.describeView = NewDescribeModel(pod.Name, pod)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceDeployments:
			if dep := m.deploymentsView.GetSelectedDeployment(); dep != nil {
				m.describeView = NewDescribeModel(dep.Name, dep)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceServices:
			if svc := m.servicesView.GetSelectedService(); svc != nil {
				m.describeView = NewDescribeModel(svc.Name, svc)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceConfigMaps:
			if cm := m.configmapsView.GetSelectedConfigMap(); cm != nil {
				m.describeView = NewDescribeModel(cm.Name, cm)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceSecrets:
			if secret := m.secretsView.GetSelectedSecret(); secret != nil {
				m.describeView = NewDescribeModelForSecret(secret.Name, secret)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceNamespaces:
			if ns := m.namespacesView.GetSelectedNamespaceObj(); ns != nil {
				m.describeView = NewDescribeModel(ns.Name, ns)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceNodes:
			if node := m.nodesView.GetSelectedNode(); node != nil {
				m.describeView = NewDescribeModel(node.Name, node)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceStatefulSets:
			if ss := m.statefulSetsView.GetSelectedStatefulSet(); ss != nil {
				m.describeView = NewDescribeModel(ss.Name, ss)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		case ResourceDaemonSets:
			if ds := m.daemonSetsView.GetSelectedDaemonSet(); ds != nil {
				m.describeView = NewDescribeModel(ds.Name, ds)
				m.describeView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewDescribe
			}
		}
		return m, nil
	}

	// Resource-specific keys
	switch m.currentResource {
	case ResourcePods:
		if key.Matches(msg, m.keys.Enter) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.relationsView = NewRelationsModel(m.client, pod)
				m.relationsView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewRelations
				return m, m.relationsView.Init()
			}
		}
		if key.Matches(msg, m.keys.Logs) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.logsView = NewLogsModel(m.client, pod)
				m.logsView.SetSize(m.width-27, m.height-7)
				m.currentView = ViewLogs
				return m, m.logsView.Init()
			}
		}
		if key.Matches(msg, m.keys.Shell) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				containerName := ""
				if len(pod.Spec.Containers) > 0 {
					containerName = pod.Spec.Containers[0].Name
				}
				execCmd := m.client.ExecCmd(pod.Name, containerName, pod.Namespace)
				return m, tea.ExecProcess(execCmd, func(err error) tea.Msg {
					return ExecFinishedMsg{Err: err}
				})
			}
		}
		if key.Matches(msg, m.keys.Delete) {
			if pod := m.podsView.GetSelectedPod(); pod != nil {
				m.confirmDelete = true
				m.confirmDeleteTarget = pod.Name
				return m, nil
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
				m.statefulSetsView.SetShowAllNamespaces(false)
				m.daemonSetsView.SetShowAllNamespaces(false)
				// Refresh all views
				return m, tea.Batch(
					m.podsView.Init(),
					m.deploymentsView.Init(),
					m.servicesView.Init(),
					m.configmapsView.Init(),
					m.secretsView.Init(),
					m.statefulSetsView.Init(),
					m.daemonSetsView.Init(),
					m.eventsView.Init(),
				)
			}
		}
		newView, cmd := m.namespacesView.Update(msg)
		m.namespacesView = newView.(*NamespacesModel)
		return m, cmd

	case ResourceNodes:
		newView, cmd := m.nodesView.Update(msg)
		m.nodesView = newView.(*NodesModel)
		return m, cmd

	case ResourceStatefulSets:
		newView, cmd := m.statefulSetsView.Update(msg)
		m.statefulSetsView = newView.(*StatefulSetsModel)
		return m, cmd

	case ResourceDaemonSets:
		newView, cmd := m.daemonSetsView.Update(msg)
		m.daemonSetsView = newView.(*DaemonSetsModel)
		return m, cmd

	case ResourceEvents:
		newView, cmd := m.eventsView.Update(msg)
		m.eventsView = newView.(*EventsModel)
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
	case ResourceNodes:
		newView, cmd := m.nodesView.Update(msg)
		m.nodesView = newView.(*NodesModel)
		return m, cmd
	case ResourceStatefulSets:
		newView, cmd := m.statefulSetsView.Update(msg)
		m.statefulSetsView = newView.(*StatefulSetsModel)
		return m, cmd
	case ResourceDaemonSets:
		newView, cmd := m.daemonSetsView.Update(msg)
		m.daemonSetsView = newView.(*DaemonSetsModel)
		return m, cmd
	case ResourceEvents:
		newView, cmd := m.eventsView.Update(msg)
		m.eventsView = newView.(*EventsModel)
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
	case ResourceNodes:
		return m.nodesView.Init()
	case ResourceStatefulSets:
		return m.statefulSetsView.Init()
	case ResourceDaemonSets:
		return m.daemonSetsView.Init()
	case ResourceEvents:
		return m.eventsView.Init()
	}
	return nil
}

func (m *Model) setFilterOnCurrentView() {
	switch m.currentResource {
	case ResourcePods:
		m.podsView.SetFilter(m.filterQuery)
	case ResourceDeployments:
		m.deploymentsView.SetFilter(m.filterQuery)
	case ResourceServices:
		m.servicesView.SetFilter(m.filterQuery)
	case ResourceConfigMaps:
		m.configmapsView.SetFilter(m.filterQuery)
	case ResourceSecrets:
		m.secretsView.SetFilter(m.filterQuery)
	case ResourceNamespaces:
		m.namespacesView.SetFilter(m.filterQuery)
	case ResourceNodes:
		m.nodesView.SetFilter(m.filterQuery)
	case ResourceStatefulSets:
		m.statefulSetsView.SetFilter(m.filterQuery)
	case ResourceDaemonSets:
		m.daemonSetsView.SetFilter(m.filterQuery)
	case ResourceEvents:
		m.eventsView.SetFilter(m.filterQuery)
	}
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
	case ResourceNodes:
		return m, m.nodesView.Refresh()
	case ResourceStatefulSets:
		return m, m.statefulSetsView.Refresh()
	case ResourceDaemonSets:
		return m, m.daemonSetsView.Refresh()
	case ResourceEvents:
		return m, m.eventsView.Refresh()
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	// Header with padding
	header := lipgloss.NewStyle().
		Padding(1, 0).
		Render(m.renderHeader())

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

func (m Model) renderHeader() string {
	logo := LogoStyle.Render("  YAKTUI")

	if m.connectionState != Connected {
		return logo + lipgloss.NewStyle().Foreground(ColorMuted).Render(" - Yet Another Kubernetes TUI")
	}

	sep := lipgloss.NewStyle().Foreground(ColorBorder).Render("  │  ")
	labelStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	valueStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	ctxVal := m.clusterInfo.Context
	if ctxVal == "" {
		ctxVal = "—"
	}
	clusterVal := m.clusterInfo.Cluster
	if clusterVal == "" {
		clusterVal = "—"
	}
	nsVal := m.client.Namespace()
	versionVal := m.clusterInfo.Version
	if versionVal == "" {
		versionVal = "…"
	}

	nsStyle := valueStyle
	if m.showAllNamespaces {
		nsStyle = warnStyle
		nsVal = "all"
	}

	info := labelStyle.Render("ctx ") + valueStyle.Render(ctxVal) +
		sep +
		labelStyle.Render("cluster ") + valueStyle.Render(clusterVal) +
		sep +
		labelStyle.Render("ns ") + nsStyle.Render(nsVal) +
		sep +
		labelStyle.Render("k8s ") + valueStyle.Render(versionVal)

	logoWidth := lipgloss.Width(logo)
	infoWidth := lipgloss.Width(info)
	totalWidth := m.width - 2
	gap := totalWidth - logoWidth - infoWidth
	if gap < 1 {
		gap = 1
	}
	padding := ""
	for i := 0; i < gap; i++ {
		padding += " "
	}

	return logo + padding + info
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

	sidebarHeight := m.height - 5
	if sidebarHeight < 5 {
		sidebarHeight = 5
	}

	focused := m.focusedPanel == SidebarPanel
	return RenderPanelWithTitle("Resources", items, 22, sidebarHeight, focused)
}

func (m Model) renderContent() string {
	var content string
	var title string
	var count int

	if m.contextMode {
		content = m.renderContextList()
		title = "Switch Context"
		contentWidth := m.width - 24
		if contentWidth < 20 {
			contentWidth = 20
		}
		contentHeight := m.height - 5
		if contentHeight < 5 {
			contentHeight = 5
		}
		return RenderPanelWithTitle(title, content, contentWidth, contentHeight, true)
	}

	switch m.currentView {
	case ViewDescribe:
		if m.describeView != nil {
			content = m.describeView.View()
			title = "YAML: " + m.describeView.GetTitle()
		}
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
		case ResourceNodes:
			content = m.nodesView.View()
			count = m.nodesView.Count()
		case ResourceStatefulSets:
			content = m.statefulSetsView.View()
			count = m.statefulSetsView.Count()
		case ResourceDaemonSets:
			content = m.daemonSetsView.View()
			count = m.daemonSetsView.Count()
		case ResourceEvents:
			content = m.eventsView.View()
			count = m.eventsView.Count()
		}
		// Build title with namespace info, count, and active filter
		nsInfo := ""
		if m.showAllNamespaces {
			nsInfo = "(all namespaces)"
		} else {
			nsInfo = "[" + m.client.Namespace() + "]"
		}
		if m.filterQuery != "" {
			title = fmt.Sprintf("%s %s (%d) /%s", m.currentResource.String(), nsInfo, count, m.filterQuery)
		} else {
			title = fmt.Sprintf("%s %s (%d)", m.currentResource.String(), nsInfo, count)
		}
	}

	// Account for sidebar (20) + borders (4) + horizontal margins (2)
	contentWidth := m.width - 24
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Account for header (3 with padding) + status bar (1) + borders
	contentHeight := m.height - 5
	if contentHeight < 5 {
		contentHeight = 5
	}

	focused := m.focusedPanel == ContentPanel
	return RenderPanelWithTitle(title, content, contentWidth, contentHeight, focused)
}

func (m Model) renderConnectingView(header string) string {
	content := lipgloss.NewStyle().
		Width(m.width - 4).
		Height(m.height - 7).
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
		Height(m.height - 7).
		Padding(2).
		Render(errorMsg)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (m Model) renderStatusBar() string {
	var help string
	sep := StatusSepStyle.Render("  ")

	if m.confirmDelete {
		warning := lipgloss.NewStyle().Foreground(ColorError).Background(lipgloss.Color("#1a1a1a")).Bold(true).Render("Delete pod")
		name := lipgloss.NewStyle().Foreground(ColorWarning).Background(lipgloss.Color("#1a1a1a")).Bold(true).Render(m.confirmDeleteTarget)
		confirm := StatusKeyStyle.Render("y") + StatusDescStyle.Render(" confirm")
		cancel := StatusKeyStyle.Render("n/esc") + StatusDescStyle.Render(" cancel")
		help = warning + StatusSepStyle.Render(" ") + name + StatusSepStyle.Render("?   ") + confirm + StatusSepStyle.Render("   ") + cancel
		return StatusBarStyle.Width(m.width).Render(help)
	}

	if m.commandMode {
		cursor := lipgloss.NewStyle().Foreground(ColorAccent).Background(lipgloss.Color("#1a1a1a")).Render("█")
		prompt := lipgloss.NewStyle().Foreground(ColorPrimary).Background(lipgloss.Color("#1a1a1a")).Bold(true).Render(":")
		matchStr := ""
		for _, match := range m.commandMatches {
			matchStr += sep + lipgloss.NewStyle().Foreground(ColorPrimary).Background(lipgloss.Color("#1a1a1a")).Render("["+match.label+"]")
		}
		help = prompt + StatusSepStyle.Render(" ") + StatusSepStyle.Render(m.commandInput) + cursor + matchStr + StatusSepStyle.Render("   ") +
			StatusKeyStyle.Render("enter") + StatusDescStyle.Render(" navigate") + sep +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" cancel")
		return StatusBarStyle.Width(m.width).Render(help)
	}

	if m.contextMode {
		help = StatusKeyStyle.Render("↑↓/jk") + StatusDescStyle.Render(" navigate") + sep +
			StatusKeyStyle.Render("enter") + StatusDescStyle.Render(" switch") + sep +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" cancel")
		return StatusBarStyle.Width(m.width).Render(help)
	}

	if m.filterMode {
		cursor := lipgloss.NewStyle().Foreground(ColorAccent).Background(lipgloss.Color("#1a1a1a")).Render("█")
		prompt := lipgloss.NewStyle().Foreground(ColorPrimary).Background(lipgloss.Color("#1a1a1a")).Bold(true).Render("/")
		help = prompt + StatusSepStyle.Render(" ") + StatusSepStyle.Render(m.filterQuery) + cursor + StatusSepStyle.Render("   ") +
			StatusKeyStyle.Render("↑↓/jk") + StatusDescStyle.Render(" navigate") + sep +
			StatusKeyStyle.Render("enter") + StatusDescStyle.Render(" confirm") + sep +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" clear")
		return StatusBarStyle.Width(m.width).Render(help)
	}

	switch m.currentView {
	case ViewDescribe:
		help = StatusKeyStyle.Render("↑↓/jk") + StatusDescStyle.Render(" scroll") + sep +
			StatusKeyStyle.Render("g/G") + StatusDescStyle.Render(" top/bottom") + sep +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" back") + sep +
			StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
	case ViewLogs:
		help = StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" back") + sep +
			StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
	case ViewRelations:
		help = StatusKeyStyle.Render("↑↓") + StatusDescStyle.Render(" navigate") + sep +
			StatusKeyStyle.Render("enter/l") + StatusDescStyle.Render(" logs") + sep +
			StatusKeyStyle.Render("esc") + StatusDescStyle.Render(" back") + sep +
			StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
	default:
		if m.currentResource == ResourcePods {
			help = StatusKeyStyle.Render("tab") + StatusDescStyle.Render(" switch") + sep +
				StatusKeyStyle.Render("enter") + StatusDescStyle.Render(" relations") + sep +
				StatusKeyStyle.Render("l") + StatusDescStyle.Render(" logs") + sep +
				StatusKeyStyle.Render("s") + StatusDescStyle.Render(" shell") + sep +
				StatusKeyStyle.Render("d") + StatusDescStyle.Render(" yaml") + sep +
				StatusKeyStyle.Render("a") + StatusDescStyle.Render(" all ns") + sep +
				StatusKeyStyle.Render("/") + StatusDescStyle.Render(" search") + sep +
				StatusKeyStyle.Render(":") + StatusDescStyle.Render(" cmd") + sep +
				StatusKeyStyle.Render("c") + StatusDescStyle.Render(" ctx") + sep +
				StatusKeyStyle.Render("?") + StatusDescStyle.Render(" help") + sep +
				StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
		} else {
			help = StatusKeyStyle.Render("tab") + StatusDescStyle.Render(" switch") + sep +
				StatusKeyStyle.Render("d") + StatusDescStyle.Render(" yaml") + sep +
				StatusKeyStyle.Render("a") + StatusDescStyle.Render(" all ns") + sep +
				StatusKeyStyle.Render("/") + StatusDescStyle.Render(" search") + sep +
				StatusKeyStyle.Render(":") + StatusDescStyle.Render(" cmd") + sep +
				StatusKeyStyle.Render("c") + StatusDescStyle.Render(" ctx") + sep +
				StatusKeyStyle.Render("?") + StatusDescStyle.Render(" help") + sep +
				StatusKeyStyle.Render("ctrl+c") + StatusDescStyle.Render(" quit")
		}
	}

	if m.statusMessage != "" {
		help = StatusSepStyle.Render(m.statusMessage) + StatusSepStyle.Render("  |  ") + help
	}

	return StatusBarStyle.Width(m.width).Render(help)
}

func (m Model) deletePod(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := m.client.DeletePod(ctx, name)
		return DeletePodResultMsg{Name: name, Err: err}
	}
}

func (m Model) fetchContexts() tea.Msg {
	contexts, current, err := client.ListContexts(m.kubeconfig)
	return ContextsMsg{Contexts: contexts, Current: current, Err: err}
}

func (m Model) switchContext(contextName string) tea.Cmd {
	return func() tea.Msg {
		newClient, err := client.NewWithContext(m.kubeconfig, contextName, "default")
		return ContextSwitchedMsg{NewClient: newClient, Err: err}
	}
}

func (m Model) navigateToResource(resource ResourceType) (tea.Model, tea.Cmd) {
	for i, r := range m.sidebarItems {
		if r == resource {
			m.sidebarSelected = i
			break
		}
	}
	m.currentResource = resource
	m.currentView = ViewList
	m.focusedPanel = ContentPanel
	m.filterMode = false
	m.filterQuery = ""
	return m, m.initCurrentView()
}

func (m Model) renderContextList() string {
	var sb strings.Builder
	checkStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	for i, ctx := range m.contexts {
		style := TableRowStyle
		prefix := "  "
		if i == m.contextSelected {
			style = TableSelectedStyle
			prefix = "▸ "
		}
		marker := ""
		if ctx == m.contextCurrent {
			marker = " " + checkStyle.Render("✓")
		}
		sb.WriteString(style.Render(prefix+ctx) + marker + "\n")
	}
	return sb.String()
}

func (m Model) renderHelpOverlay() string {
	keyStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorText)
	sectionStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	type row struct{ k, d string }
	sections := []struct {
		title string
		rows  []row
	}{
		{"Navigation", []row{
			{"↑/k  ↓/j", "move up / down"},
			{"←/h  →/l", "move left / right"},
			{"tab", "switch panel"},
			{"enter", "select / confirm"},
			{"esc", "back / cancel"},
		}},
		{"Resource actions", []row{
			{"l", "pod logs"},
			{"s", "shell (pods)"},
			{"d", "yaml / describe"},
			{"enter", "relations (pods)"},
			{"del", "delete pod"},
		}},
		{"Global", []row{
			{"/", "filter resources"},
			{":", "command palette"},
			{"a", "toggle all namespaces"},
			{"c", "switch context"},
			{"?", "toggle this help"},
			{"ctrl+c", "quit"},
		}},
	}

	var buf strings.Builder
	buf.WriteString(LogoStyle.Render("  Key Bindings") + "\n\n")
	for _, sec := range sections {
		buf.WriteString(sectionStyle.Render(sec.title) + "\n")
		for _, r := range sec.rows {
			kw := lipgloss.NewStyle().Width(14).Render(keyStyle.Render(r.k))
			buf.WriteString("  " + kw + descStyle.Render(r.d) + "\n")
		}
		buf.WriteString("\n")
	}
	buf.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("press ? or esc to close"))

	box := DialogStyle.Width(44).Render(buf.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// Message types
type TickMsg struct{}
type ErrorMsg struct{ Err error }
type AllNamespacesToggleMsg struct{}
type ExecFinishedMsg struct{ Err error }
type DeletePodResultMsg struct {
	Name string
	Err  error
}
type ClusterInfoMsg struct {
	Version string
}
type ContextsMsg struct {
	Contexts []string
	Current  string
	Err      error
}
type ContextSwitchedMsg struct {
	NewClient *client.Client
	Err       error
}
