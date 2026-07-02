package tui

import "testing"

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
