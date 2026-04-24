package tui

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type DescribeModel struct {
	viewport viewport.Model
	title    string
	width    int
	height   int
}

func NewDescribeModel(title string, obj interface{}) *DescribeModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Foreground(ColorText)
	vp.SetContent(objectToYAML(obj))

	return &DescribeModel{
		viewport: vp,
		title:    title,
	}
}

func NewDescribeModelForSecret(title string, secret *corev1.Secret) *DescribeModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Foreground(ColorText)
	vp.SetContent(secretToYAML(secret))

	return &DescribeModel{
		viewport: vp,
		title:    title,
	}
}

func (m *DescribeModel) Init() tea.Cmd {
	return nil
}

func (m *DescribeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 4
	m.viewport.Height = height - 3
}

func (m *DescribeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "g":
			m.viewport.GotoTop()
			return m, nil
		case "G":
			m.viewport.GotoBottom()
			return m, nil
		}
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *DescribeModel) View() string {
	scrollInfo := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(fmt.Sprintf(" %d%% ", int(m.viewport.ScrollPercent()*100)))
	return scrollInfo + "\n" + m.viewport.View()
}

func (m *DescribeModel) GetTitle() string {
	return m.title
}

func objectToYAML(obj interface{}) string {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return "error: " + err.Error()
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err == nil {
		if meta, ok := raw["metadata"].(map[string]interface{}); ok {
			delete(meta, "managedFields")
		}
		jsonData, _ = json.Marshal(raw)
	}

	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(yamlData)
}

func secretToYAML(secret *corev1.Secret) string {
	jsonData, err := json.Marshal(secret)
	if err != nil {
		return "error: " + err.Error()
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err == nil {
		if meta, ok := raw["metadata"].(map[string]interface{}); ok {
			delete(meta, "managedFields")
		}
		if data, ok := raw["data"].(map[string]interface{}); ok {
			for k := range data {
				data[k] = "[REDACTED]"
			}
		}
		jsonData, _ = json.Marshal(raw)
	}

	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(yamlData)
}
