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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// RelationItem represents an item in the relations graph
type RelationItem struct {
	Kind string
	Name string
}

// PodRelations holds the related resources for a pod
type PodRelations struct {
	Pod        *corev1.Pod
	ReplicaSet *appsv1.ReplicaSet
	Deployment *appsv1.Deployment
	Services   []corev1.Service
	ConfigMaps []string
	Secrets    []string
}

// RelationsModel represents the relations view
type RelationsModel struct {
	client    *client.Client
	pod       *corev1.Pod
	relations *PodRelations
	items     []RelationItem
	selected  int
	width     int
	height    int
	loading   bool
}

// NewRelationsModel creates a new relations model
func NewRelationsModel(c *client.Client, pod *corev1.Pod) *RelationsModel {
	return &RelationsModel{
		client:   c,
		pod:      pod,
		selected: 0,
		loading:  true,
	}
}

// Init initializes the model
func (m *RelationsModel) Init() tea.Cmd {
	return m.fetchRelations
}

// SetSize sets the view size
func (m *RelationsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages
func (m *RelationsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, DefaultKeyMap.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, DefaultKeyMap.Down):
			if m.selected < len(m.items)-1 {
				m.selected++
			}
		}

	case RelationsMsg:
		m.relations = msg.Relations
		m.loading = false
		m.buildItems()
		return m, nil
	}

	return m, nil
}

// View renders the model
func (m *RelationsModel) View() string {
	if m.loading {
		return "Loading relations..."
	}

	graph := m.renderGraph()
	details := m.renderDetails()

	// Use sensible defaults if size not set
	width := m.width
	height := m.height
	if width < 40 {
		width = 80
	}
	if height < 10 {
		height = 20
	}

	// Split view horizontally
	halfWidth := width/2 - 2
	contentHeight := height - 4

	graphStyle := lipgloss.NewStyle().
		Width(halfWidth).
		Height(contentHeight).
		Padding(1)

	detailsStyle := lipgloss.NewStyle().
		Width(halfWidth).
		Height(contentHeight).
		Padding(1).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		graphStyle.Render(graph),
		detailsStyle.Render(details),
	)
}

// GetPodName returns the pod name for the title
func (m *RelationsModel) GetPodName() string {
	if m.pod != nil {
		return m.pod.Name
	}
	return ""
}

// GetSelectedPod returns the pod if currently selected
func (m *RelationsModel) GetSelectedPod() *corev1.Pod {
	if m.selected < len(m.items) && m.items[m.selected].Kind == "Pod" {
		return m.pod
	}
	return nil
}

func (m *RelationsModel) fetchRelations() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relations := &PodRelations{Pod: m.pod}
	m.client.SetNamespace(m.pod.Namespace)

	// Find owner ReplicaSet
	for _, owner := range m.pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			rs, err := m.findReplicaSet(ctx, owner.Name)
			if err == nil && rs != nil {
				relations.ReplicaSet = rs
				// Find owner Deployment
				for _, rsOwner := range rs.OwnerReferences {
					if rsOwner.Kind == "Deployment" {
						dep, err := m.client.GetDeployment(ctx, rsOwner.Name)
						if err == nil {
							relations.Deployment = dep
						}
					}
				}
			}
		}
	}

	// Find services that select this pod
	services, err := m.client.ListServices(ctx)
	if err == nil {
		for _, svc := range services {
			if m.serviceSelectsPod(&svc, m.pod) {
				relations.Services = append(relations.Services, svc)
			}
		}
	}

	// Find ConfigMaps and Secrets used by the pod
	for _, vol := range m.pod.Spec.Volumes {
		if vol.ConfigMap != nil {
			relations.ConfigMaps = append(relations.ConfigMaps, vol.ConfigMap.Name)
		}
		if vol.Secret != nil {
			relations.Secrets = append(relations.Secrets, vol.Secret.SecretName)
		}
	}

	// Check env vars for ConfigMaps and Secrets
	for _, container := range m.pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				relations.ConfigMaps = append(relations.ConfigMaps, envFrom.ConfigMapRef.Name)
			}
			if envFrom.SecretRef != nil {
				relations.Secrets = append(relations.Secrets, envFrom.SecretRef.Name)
			}
		}
	}

	return RelationsMsg{Relations: relations}
}

func (m *RelationsModel) findReplicaSet(ctx context.Context, name string) (*appsv1.ReplicaSet, error) {
	rsList, err := m.client.ListReplicaSets(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rsList {
		if rsList[i].Name == name {
			return &rsList[i], nil
		}
	}
	return nil, nil
}

func (m *RelationsModel) serviceSelectsPod(svc *corev1.Service, pod *corev1.Pod) bool {
	if len(svc.Spec.Selector) == 0 {
		return false
	}
	for key, value := range svc.Spec.Selector {
		if podValue, exists := pod.Labels[key]; !exists || podValue != value {
			return false
		}
	}
	return true
}

func (m *RelationsModel) buildItems() {
	m.items = nil

	// Pod first (main focus)
	m.items = append(m.items, RelationItem{Kind: "Pod", Name: m.pod.Name})

	// Ownership chain (ReplicaSet, Deployment)
	if m.relations.ReplicaSet != nil {
		m.items = append(m.items, RelationItem{Kind: "ReplicaSet", Name: m.relations.ReplicaSet.Name})
	}
	if m.relations.Deployment != nil {
		m.items = append(m.items, RelationItem{Kind: "Deployment", Name: m.relations.Deployment.Name})
	}

	// Related resources
	for _, svc := range m.relations.Services {
		m.items = append(m.items, RelationItem{Kind: "Service", Name: svc.Name})
	}
	for _, cm := range m.relations.ConfigMaps {
		m.items = append(m.items, RelationItem{Kind: "ConfigMap", Name: cm})
	}
	for _, secret := range m.relations.Secrets {
		m.items = append(m.items, RelationItem{Kind: "Secret", Name: secret})
	}
}

func (m *RelationsModel) renderGraph() string {
	if m.relations == nil {
		return "No relations found"
	}

	var sb strings.Builder
	idx := 0

	// Build pod box content
	var boxContent strings.Builder

	// Pod (main focus)
	boxContent.WriteString(m.formatNode("Pod", m.pod.Name, idx))
	idx++

	// Services inside box
	if len(m.relations.Services) > 0 {
		boxContent.WriteString("     │\n")
		for i, svc := range m.relations.Services {
			if i == len(m.relations.Services)-1 {
				boxContent.WriteString("     └── ")
			} else {
				boxContent.WriteString("     ├── ")
			}
			boxContent.WriteString(m.formatNodeInline("Service", svc.Name, idx) + "\n")
			idx++
		}
	}

	// ConfigMaps inside box
	if len(m.relations.ConfigMaps) > 0 {
		boxContent.WriteString("     │\n")
		for i, cm := range m.relations.ConfigMaps {
			if i == len(m.relations.ConfigMaps)-1 && len(m.relations.Secrets) == 0 {
				boxContent.WriteString("     └── ")
			} else {
				boxContent.WriteString("     ├── ")
			}
			boxContent.WriteString(m.formatNodeInline("ConfigMap", cm, idx) + "\n")
			idx++
		}
	}

	// Secrets inside box
	if len(m.relations.Secrets) > 0 {
		if len(m.relations.ConfigMaps) == 0 && len(m.relations.Services) == 0 {
			boxContent.WriteString("     │\n")
		}
		for i, secret := range m.relations.Secrets {
			if i == len(m.relations.Secrets)-1 {
				boxContent.WriteString("     └── ")
			} else {
				boxContent.WriteString("     ├── ")
			}
			boxContent.WriteString(m.formatNodeInline("Secret", secret, idx) + "\n")
			idx++
		}
	}

	// Create box around pod and its connections
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	sb.WriteString(boxStyle.Render(boxContent.String()))
	sb.WriteString("\n")

	// Ownership chain outside box (going down)
	if m.relations.ReplicaSet != nil || m.relations.Deployment != nil {
		sb.WriteString("       │\n")
		sb.WriteString("       ⌄\n")
	}

	if m.relations.ReplicaSet != nil {
		sb.WriteString("    " + m.formatNodeInline("ReplicaSet", m.relations.ReplicaSet.Name, idx) + "\n")
		idx++
		if m.relations.Deployment != nil {
			sb.WriteString("       │\n")
			sb.WriteString("       ⌄\n")
		}
	}

	if m.relations.Deployment != nil {
		sb.WriteString("    " + m.formatNodeInline("Deployment", m.relations.Deployment.Name, idx) + "\n")
		idx++
	}

	return sb.String()
}

func (m *RelationsModel) getKindStyle(kind string) lipgloss.Style {
	switch kind {
	case "Deployment":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#5F87FF"))
	case "ReplicaSet":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#AF87FF"))
	case "Pod":
		return lipgloss.NewStyle().Foreground(ColorSuccess)
	case "Service":
		return lipgloss.NewStyle().Foreground(ColorPrimary)
	case "ConfigMap":
		return lipgloss.NewStyle().Foreground(ColorAccent)
	case "Secret":
		return lipgloss.NewStyle().Foreground(ColorError)
	default:
		return lipgloss.NewStyle().Foreground(ColorText)
	}
}

func (m *RelationsModel) formatNode(kind, name string, idx int) string {
	kindStyle := m.getKindStyle(kind)

	prefix := "  "
	if idx == m.selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#005F87")).
			Bold(true).
			Render(fmt.Sprintf(" ▸ %s: %s ", kind, name)) + "\n"
	}
	return prefix + kindStyle.Render(kind+": ") + name + "\n"
}

func (m *RelationsModel) formatNodeInline(kind, name string, idx int) string {
	kindStyle := m.getKindStyle(kind)

	if idx == m.selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#005F87")).
			Bold(true).
			Render(fmt.Sprintf("▸ %s: %s", kind, name))
	}
	return kindStyle.Render(kind+": ") + name
}

func (m *RelationsModel) renderDetails() string {
	if m.selected >= len(m.items) {
		return ""
	}

	item := m.items[m.selected]
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	switch item.Kind {
	case "Pod":
		sb.WriteString(titleStyle.Render("Pod: "+item.Name) + "\n\n")
		sb.WriteString(labelStyle.Render("Namespace: ") + m.pod.Namespace + "\n")
		sb.WriteString(labelStyle.Render("Status: ") + StatusText(string(m.pod.Status.Phase)) + "\n")
		sb.WriteString(labelStyle.Render("IP: ") + m.pod.Status.PodIP + "\n")
		sb.WriteString(labelStyle.Render("Node: ") + m.pod.Spec.NodeName + "\n")
		sb.WriteString(labelStyle.Render("Age: ") + formatAge(m.pod.CreationTimestamp.Time) + "\n")
		sb.WriteString("\n" + labelStyle.Render("Containers:") + "\n")
		for _, c := range m.pod.Spec.Containers {
			sb.WriteString(fmt.Sprintf("  • %s (%s)\n", c.Name, c.Image))
		}

	case "Deployment":
		if m.relations.Deployment != nil {
			dep := m.relations.Deployment
			sb.WriteString(titleStyle.Render("Deployment: "+item.Name) + "\n\n")
			var replicas int32 = 0
			if dep.Spec.Replicas != nil {
				replicas = *dep.Spec.Replicas
			}
			sb.WriteString(labelStyle.Render("Replicas: ") + fmt.Sprintf("%d/%d\n", dep.Status.ReadyReplicas, replicas))
			sb.WriteString(labelStyle.Render("Strategy: ") + string(dep.Spec.Strategy.Type) + "\n")
			sb.WriteString(labelStyle.Render("Age: ") + formatAge(dep.CreationTimestamp.Time) + "\n")
		}

	case "ReplicaSet":
		if m.relations.ReplicaSet != nil {
			rs := m.relations.ReplicaSet
			sb.WriteString(titleStyle.Render("ReplicaSet: "+item.Name) + "\n\n")
			var replicas int32 = 0
			if rs.Spec.Replicas != nil {
				replicas = *rs.Spec.Replicas
			}
			sb.WriteString(labelStyle.Render("Replicas: ") + fmt.Sprintf("%d/%d\n", rs.Status.ReadyReplicas, replicas))
			sb.WriteString(labelStyle.Render("Age: ") + formatAge(rs.CreationTimestamp.Time) + "\n")
		}

	case "Service":
		for _, svc := range m.relations.Services {
			if svc.Name == item.Name {
				sb.WriteString(titleStyle.Render("Service: "+item.Name) + "\n\n")
				sb.WriteString(labelStyle.Render("Type: ") + string(svc.Spec.Type) + "\n")
				sb.WriteString(labelStyle.Render("ClusterIP: ") + svc.Spec.ClusterIP + "\n")
				sb.WriteString(labelStyle.Render("Ports:") + "\n")
				for _, port := range svc.Spec.Ports {
					sb.WriteString(fmt.Sprintf("  • %s: %d → %d\n", port.Name, port.Port, port.TargetPort.IntValue()))
				}
				break
			}
		}

	case "ConfigMap":
		sb.WriteString(titleStyle.Render("ConfigMap: "+item.Name) + "\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("Used by this pod\n"))

	case "Secret":
		sb.WriteString(titleStyle.Render("Secret: "+item.Name) + "\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("Used by this pod\n"))
	}

	return sb.String()
}

// Message types
type RelationsMsg struct {
	Relations *PodRelations
}
