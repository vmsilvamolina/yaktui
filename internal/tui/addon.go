package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/addons"
	"github.com/vmsilvamolina/yaktui/internal/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AddonModel is a generic list view for any CRD-backed addons.Definition,
// following the same fetch/cache/filter contract as the built-in resource
// models (see EventsModel) but driven entirely by the definition's GVR and
// column config instead of a typed clientset method per resource.
type AddonModel struct {
	def               addons.Definition
	client            *client.Client
	table             table.Model
	items             []unstructured.Unstructured
	visibleItems      []unstructured.Unstructured
	showAllNamespaces bool
	width             int
	height            int
	loading           bool
	filterQuery       string
	filteredCount     int
	itemsAllNS        []unstructured.Unstructured
	itemsSingleNS     []unstructured.Unstructured
	cacheNS           string
}

func addonTableStylesNormal() table.Styles {
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

func addonColumnHeaders(def addons.Definition) []string {
	var headers []string
	if len(def.Resources) > 1 {
		headers = append(headers, "KIND")
	}
	if def.HasNamespacedResource() {
		headers = append(headers, "NAMESPACE")
	}
	for _, c := range def.Columns {
		headers = append(headers, c.Header)
	}
	return headers
}

func NewAddonModel(def addons.Definition, c *client.Client) *AddonModel {
	headers := addonColumnHeaders(def)
	columns := make([]table.Column, len(headers))
	for i, h := range headers {
		columns[i] = table.Column{Title: h, Width: 20}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(addonTableStylesNormal())

	return &AddonModel{
		def:     def,
		client:  c,
		table:   t,
		loading: true,
	}
}

func (m *AddonModel) Init() tea.Cmd {
	return m.fetch
}

// InitAllNamespaces preloads the all-namespaces cache. A definition made up
// entirely of cluster-scoped resources has no namespace concept, so there's
// nothing extra to preload.
func (m *AddonModel) InitAllNamespaces() tea.Cmd {
	if !m.def.HasNamespacedResource() {
		return nil
	}
	return m.fetchAllNS
}

// SetShowAllNamespaces is a no-op when the definition has no namespaced
// resources.
func (m *AddonModel) SetShowAllNamespaces(showAll bool) {
	if !m.def.HasNamespacedResource() {
		return
	}
	m.showAllNamespaces = showAll
	if showAll && len(m.itemsAllNS) > 0 {
		m.items = m.itemsAllNS
		m.updateTable()
	} else if !showAll && len(m.itemsSingleNS) > 0 && m.cacheNS == m.client.Namespace() {
		m.items = m.itemsSingleNS
		m.updateTable()
	}
}

func (m *AddonModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

func (m *AddonModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	headers := addonColumnHeaders(m.def)
	// Each column gets 1 char of horizontal padding on both sides
	// (table.DefaultStyles()), so that overhead must come out of the
	// budget before dividing it evenly across columns.
	total := width - 10 - 2*len(headers)
	if total < len(headers) {
		total = len(headers)
	}
	colWidth := total / len(headers)

	columns := make([]table.Column, len(headers))
	for i, h := range headers {
		columns[i] = table.Column{Title: h, Width: colWidth}
	}
	m.table.SetColumns(columns)
}

func (m *AddonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case AddonMsg:
		if msg.Name != m.def.Name {
			break
		}
		m.items = msg.Items
		m.loading = false
		if m.showAllNamespaces {
			m.itemsAllNS = msg.Items
		} else {
			m.itemsSingleNS = msg.Items
			m.cacheNS = m.client.Namespace()
		}
		m.updateTable()
		return m, nil

	case AddonAllNSMsg:
		if msg.Name != m.def.Name {
			break
		}
		m.itemsAllNS = msg.Items
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *AddonModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

func (m *AddonModel) Count() int {
	return m.filteredCount
}

func (m *AddonModel) Refresh() tea.Cmd {
	return m.fetch
}

// GetSelected returns the item behind the currently selected table row.
func (m *AddonModel) GetSelected() *unstructured.Unstructured {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.visibleItems) {
		return nil
	}
	item := m.visibleItems[idx]
	return &item
}

// fetchItems lists every Resource in the definition and merges the results,
// tagging each item with its Resource's Kind so mixed rows (e.g. Kyverno's
// ClusterPolicy + Policy) can be told apart in the KIND column.
func (m *AddonModel) fetchItems(ctx context.Context, allNamespaces bool) ([]unstructured.Unstructured, error) {
	var items []unstructured.Unstructured

	for _, r := range m.def.Resources {
		gvr := m.def.GVR(r)

		var list *unstructured.UnstructuredList
		var err error
		switch {
		case r.ClusterScoped:
			list, err = m.client.ListAddonResource(ctx, gvr, false)
		case allNamespaces:
			list, err = m.client.ListAllAddonResource(ctx, gvr)
		default:
			list, err = m.client.ListAddonResource(ctx, gvr, true)
		}
		if err != nil {
			return nil, err
		}

		for i := range list.Items {
			list.Items[i].SetKind(r.Kind)
		}
		items = append(items, list.Items...)
	}

	return items, nil
}

func (m *AddonModel) fetch() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	items, err := m.fetchItems(ctx, m.showAllNamespaces)
	if err != nil {
		return ErrorMsg{Err: err}
	}
	return AddonMsg{Name: m.def.Name, Items: items}
}

func (m *AddonModel) fetchAllNS() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	items, err := m.fetchItems(ctx, true)
	if err != nil {
		return ErrorMsg{Err: err}
	}
	return AddonAllNSMsg{Name: m.def.Name, Items: items}
}

func (m *AddonModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row
	var visible []unstructured.Unstructured

	for _, item := range m.items {
		if q != "" && !strings.Contains(strings.ToLower(item.GetName()), q) {
			continue
		}

		var row table.Row
		if len(m.def.Resources) > 1 {
			row = append(row, item.GetKind())
		}
		if m.def.HasNamespacedResource() {
			row = append(row, item.GetNamespace())
		}
		for _, col := range m.def.Columns {
			if col.Path == "metadata.creationTimestamp" {
				row = append(row, formatAgeFromRFC3339(addons.ExtractColumn(item.Object, col.Path)))
				continue
			}
			row = append(row, addons.ExtractColumn(item.Object, col.Path))
		}

		rows = append(rows, row)
		visible = append(visible, item)
	}

	m.filteredCount = len(rows)
	m.visibleItems = visible
	m.table.SetRows(rows)
}

func formatAgeFromRFC3339(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return "?"
	}
	return formatAge(t)
}

type AddonMsg struct {
	Name  string
	Items []unstructured.Unstructured
}

type AddonAllNSMsg struct {
	Name  string
	Items []unstructured.Unstructured
}

// addonMsgName extracts the addon Name from an AddonMsg or AddonAllNSMsg,
// used to route the message to the right AddonModel instance.
func addonMsgName(msg tea.Msg) string {
	switch m := msg.(type) {
	case AddonMsg:
		return m.Name
	case AddonAllNSMsg:
		return m.Name
	}
	return ""
}
