package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRefreshKeyBindingMatchesLowerR(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	if !key.Matches(msg, DefaultKeyMap.Refresh) {
		t.Fatal("expected Refresh binding to match 'r'")
	}
}

func TestManualRefreshInListViewSetsStatusAndCmd(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewModel(c, "/tmp/kubeconfig", nil)
	m.connectionState = Connected
	m.currentView = ViewList
	m.currentResource = ResourcePods

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	updated := newModel.(Model)

	if updated.statusMessage != "refreshing Pods" {
		t.Fatalf("expected status %q, got %q", "refreshing Pods", updated.statusMessage)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil command from manual refresh")
	}
}

func TestManualRefreshIgnoredOutsideListView(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewModel(c, "/tmp/kubeconfig", nil)
	m.connectionState = Connected
	m.currentView = ViewDescribe
	m.currentResource = ResourcePods

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	updated := newModel.(Model)

	if updated.statusMessage == "refreshing Pods" {
		t.Fatal("expected manual refresh to be ignored when not in list view")
	}
}
