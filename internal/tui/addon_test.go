package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vmsilvamolina/yaktui/internal/addons"
	"github.com/vmsilvamolina/yaktui/internal/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const addonTestKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: test-context
  context:
    cluster: test-cluster
current-context: test-context
users: []
`

func newAddonTestClient(t *testing.T, namespace string) *client.Client {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(path, []byte(addonTestKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(path, namespace)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// clusterScopedOnlyDef is a single-resource, cluster-scoped addon (like
// Kyverno's ClusterPolicy on its own).
func clusterScopedOnlyDef() addons.Definition {
	return addons.Definition{
		Name:        "kyverno-clusterpolicies",
		DisplayName: "Cl.Policies",
		Group:       "kyverno.io",
		Version:     "v1",
		Resources: []addons.Resource{
			{Kind: "ClusterPolicy", Resource: "clusterpolicies", ClusterScoped: true},
		},
		Columns: []addons.Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "BACKGROUND", Path: "spec.background"},
			{Header: "AGE", Path: "metadata.creationTimestamp"},
		},
	}
}

// singleNamespacedDef is a single-resource, namespaced addon (like Kyverno's
// Policy on its own).
func singleNamespacedDef() addons.Definition {
	return addons.Definition{
		Name:        "kyverno-policies",
		DisplayName: "Policies",
		Group:       "kyverno.io",
		Version:     "v1",
		Resources: []addons.Resource{
			{Kind: "Policy", Resource: "policies", ClusterScoped: false},
		},
		Columns: []addons.Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "BACKGROUND", Path: "spec.background"},
		},
	}
}

// kyvernoMixedDef mirrors the shipped Kyverno addon: one Definition merging
// a cluster-scoped and a namespaced resource into a single sidebar entry.
func kyvernoMixedDef() addons.Definition {
	return addons.Definition{
		Name:        "kyverno",
		DisplayName: "Kyverno",
		Group:       "kyverno.io",
		Version:     "v1",
		Resources: []addons.Resource{
			{Kind: "ClusterPolicy", Resource: "clusterpolicies", ClusterScoped: true},
			{Kind: "Policy", Resource: "policies", ClusterScoped: false},
		},
		Columns: []addons.Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "BACKGROUND", Path: "spec.background"},
		},
	}
}

func TestAddonColumnHeaders(t *testing.T) {
	got := addonColumnHeaders(clusterScopedOnlyDef())
	want := []string{"NAME", "BACKGROUND", "AGE"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	got = addonColumnHeaders(singleNamespacedDef())
	want = []string{"NAMESPACE", "NAME", "BACKGROUND"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	got = addonColumnHeaders(kyvernoMixedDef())
	want = []string{"KIND", "NAMESPACE", "NAME", "BACKGROUND"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNewAddonModel(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)
	if !m.loading {
		t.Fatal("expected new addon model to start loading")
	}
	if len(m.table.Columns()) != 3 {
		t.Fatalf("expected 3 columns (NAMESPACE+NAME+BACKGROUND), got %d", len(m.table.Columns()))
	}
}

// TestAddonModelSetSizeAccountsForColumnPadding guards against the header
// row rendering wider than the panel it's placed in: table.DefaultStyles()
// adds 1 char of padding on each side of every column, which SetSize must
// subtract from its width budget before dividing it across columns.
func TestAddonModelSetSizeAccountsForColumnPadding(t *testing.T) {
	c := newAddonTestClient(t, "default")
	for _, def := range []addons.Definition{kyvernoMixedDef(), singleNamespacedDef(), clusterScopedOnlyDef()} {
		width := 173
		m := NewAddonModel(def, c)
		m.SetSize(width, 30)

		lines := splitLines(m.table.View())
		if len(lines) < 2 {
			t.Fatalf("%s: expected at least a header line, got %d lines", def.DisplayName, len(lines))
		}
		if got := lipgloss.Width(lines[1]); got > width-10 {
			t.Errorf("%s: header line width %d exceeds budget %d", def.DisplayName, got, width-10)
		}
	}
}

func newUnstructuredItem(name, namespace, background string) unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "kyverno.io/v1",
		"kind":       "Policy",
		"metadata": map[string]interface{}{
			"name":              name,
			"creationTimestamp": "2020-01-01T00:00:00Z",
		},
		"spec": map[string]interface{}{
			"background": background,
		},
	}
	if namespace != "" {
		obj["metadata"].(map[string]interface{})["namespace"] = namespace
	}
	return unstructured.Unstructured{Object: obj}
}

func TestAddonModelUpdateSetsItemsAndClearsLoading(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)
	m.SetSize(120, 30)

	newModel, _ := m.Update(AddonMsg{
		Name: "kyverno-policies",
		Items: []unstructured.Unstructured{
			newUnstructuredItem("pol-a", "default", "true"),
		},
	})
	m = newModel.(*AddonModel)

	if m.loading {
		t.Fatal("expected loading to be false after AddonMsg")
	}
	if m.Count() != 1 {
		t.Fatalf("expected count 1, got %d", m.Count())
	}
	if m.View() == "Loading..." {
		t.Fatal("expected non-loading view")
	}
}

func TestAddonModelUpdateIgnoresOtherAddonName(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)

	newModel, _ := m.Update(AddonMsg{
		Name:  "some-other-addon",
		Items: []unstructured.Unstructured{newUnstructuredItem("pol-a", "default", "true")},
	})
	m = newModel.(*AddonModel)

	if !m.loading {
		t.Fatal("expected loading to remain true for unrelated addon message")
	}
	if m.Count() != 0 {
		t.Fatalf("expected count 0, got %d", m.Count())
	}
}

func TestAddonModelFilter(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)
	m.SetSize(120, 30)

	newModel, _ := m.Update(AddonMsg{
		Name: "kyverno-policies",
		Items: []unstructured.Unstructured{
			newUnstructuredItem("pol-a", "default", "true"),
			newUnstructuredItem("pol-b", "default", "false"),
		},
	})
	m = newModel.(*AddonModel)

	m.SetFilter("pol-a")
	if m.Count() != 1 {
		t.Fatalf("expected count 1 after filter, got %d", m.Count())
	}

	m.SetFilter("")
	if m.Count() != 2 {
		t.Fatalf("expected count 2 after clearing filter, got %d", m.Count())
	}
}

func TestAddonModelGetSelected(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)
	m.SetSize(120, 30)

	newModel, _ := m.Update(AddonMsg{
		Name: "kyverno-policies",
		Items: []unstructured.Unstructured{
			newUnstructuredItem("pol-a", "default", "true"),
		},
	})
	m = newModel.(*AddonModel)

	sel := m.GetSelected()
	if sel == nil {
		t.Fatal("expected a selected item")
	}
	if sel.GetName() != "pol-a" {
		t.Fatalf("expected pol-a, got %s", sel.GetName())
	}
}

func TestAddonModelGetSelectedEmpty(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)

	if sel := m.GetSelected(); sel != nil {
		t.Fatalf("expected nil selection on empty table, got %+v", sel)
	}
}

func TestAddonModelClusterScopedNoOpAllNamespaces(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(clusterScopedOnlyDef(), c)

	if cmd := m.InitAllNamespaces(); cmd != nil {
		t.Fatal("expected InitAllNamespaces to be a no-op for cluster-scoped addon")
	}

	m.itemsAllNS = []unstructured.Unstructured{newUnstructuredItem("cp-a", "", "true")}
	m.SetShowAllNamespaces(true)
	if m.showAllNamespaces {
		t.Fatal("expected SetShowAllNamespaces to be a no-op for cluster-scoped addon")
	}
}

func TestAddonModelInitAllNamespacesNamespaced(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)

	if cmd := m.InitAllNamespaces(); cmd == nil {
		t.Fatal("expected InitAllNamespaces to return a fetch command for namespaced addon")
	}
}

func TestAddonModelInitAllNamespacesMixed(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(kyvernoMixedDef(), c)

	if cmd := m.InitAllNamespaces(); cmd == nil {
		t.Fatal("expected InitAllNamespaces to return a fetch command when any resource is namespaced")
	}
}

func newUnstructuredKindItem(kind, name, namespace string) unstructured.Unstructured {
	item := newUnstructuredItem(name, namespace, "true")
	item.SetKind(kind)
	return item
}

func TestAddonModelMixedKindColumn(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(kyvernoMixedDef(), c)
	m.SetSize(120, 30)

	newModel, _ := m.Update(AddonMsg{
		Name: "kyverno",
		Items: []unstructured.Unstructured{
			newUnstructuredKindItem("ClusterPolicy", "cpol-a", ""),
			newUnstructuredKindItem("Policy", "pol-a", "default"),
		},
	})
	m = newModel.(*AddonModel)

	if m.Count() != 2 {
		t.Fatalf("expected 2 merged rows, got %d", m.Count())
	}

	m.table.GotoTop()
	first := m.GetSelected()
	if first == nil || first.GetKind() != "ClusterPolicy" {
		t.Fatalf("expected first row to be a ClusterPolicy, got %+v", first)
	}
}

func TestFormatAgeFromRFC3339(t *testing.T) {
	if got := formatAgeFromRFC3339("not-a-timestamp"); got != "?" {
		t.Fatalf("expected ? for invalid timestamp, got %q", got)
	}
	if got := formatAgeFromRFC3339("2020-01-01T00:00:00Z"); got == "?" {
		t.Fatalf("expected a formatted age, got %q", got)
	}
}

func TestAddonModelRefreshReturnsFetchCmd(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewAddonModel(singleNamespacedDef(), c)

	if cmd := m.Refresh(); cmd == nil {
		t.Fatal("expected Refresh to return a non-nil command")
	}
}
