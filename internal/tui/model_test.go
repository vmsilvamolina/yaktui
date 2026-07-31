package tui

import (
	"testing"

	"github.com/vmsilvamolina/yaktui/internal/addons"
)

func TestNewModelWiresAddonsIntoSidebar(t *testing.T) {
	c := newAddonTestClient(t, "default")
	defs := []addons.Definition{clusterScopedOnlyDef(), singleNamespacedDef()}

	m := NewModel(c, "/tmp/kubeconfig", defs)

	if len(m.addonModels) != 2 {
		t.Fatalf("expected 2 addon models, got %d", len(m.addonModels))
	}
	if len(m.sidebarItems) != len(k8sResourceTypes)+2 {
		t.Fatalf("expected sidebar to include addons, got %d items", len(m.sidebarItems))
	}

	lastTwo := m.sidebarItems[len(m.sidebarItems)-2:]
	if m.resourceLabel(lastTwo[0]) != "Cl.Policies" || m.resourceLabel(lastTwo[1]) != "Policies" {
		t.Fatalf("unexpected addon labels: %q, %q", m.resourceLabel(lastTwo[0]), m.resourceLabel(lastTwo[1]))
	}
}

func TestNewModelSingleMixedAddonProducesOneSidebarEntry(t *testing.T) {
	c := newAddonTestClient(t, "default")
	defs := []addons.Definition{kyvernoMixedDef()}

	m := NewModel(c, "/tmp/kubeconfig", defs)

	if len(m.addonModels) != 1 {
		t.Fatalf("expected 1 addon model, got %d", len(m.addonModels))
	}
	if len(m.sidebarItems) != len(k8sResourceTypes)+1 {
		t.Fatalf("expected sidebar to include exactly 1 addon entry, got %d items", len(m.sidebarItems))
	}

	last := m.sidebarItems[len(m.sidebarItems)-1]
	if m.resourceLabel(last) != "Kyverno" {
		t.Fatalf("unexpected addon label: %q", m.resourceLabel(last))
	}
}

func TestResourceLabelBuiltin(t *testing.T) {
	m := Model{}
	if got := m.resourceLabel(ResourcePods); got != "Pods" {
		t.Errorf("resourceLabel(ResourcePods) = %q, want %q", got, "Pods")
	}
}

func TestResourceLabelUnknownAddon(t *testing.T) {
	m := Model{addonByType: map[ResourceType]*AddonModel{}}
	if got := m.resourceLabel(resourceAddonBase); got != "Unknown" {
		t.Errorf("resourceLabel() = %q, want %q", got, "Unknown")
	}
}

func TestClearStatusMsgClearsStatusMessage(t *testing.T) {
	m := Model{statusMessage: "deleted pod: foo"}

	newModel, cmd := m.Update(ClearStatusMsg{})

	updated := newModel.(Model)
	if updated.statusMessage != "" {
		t.Errorf("expected statusMessage to be cleared, got %q", updated.statusMessage)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd, got non-nil")
	}
}
