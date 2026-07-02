package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestQuitKeyBindingMatchesCtrlCAndQ(t *testing.T) {
	for _, k := range []string{"ctrl+c", "q"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "ctrl+c" {
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		}
		if !key.Matches(msg, DefaultKeyMap.Quit) {
			t.Errorf("expected Quit binding to match %q", k)
		}
	}
}
