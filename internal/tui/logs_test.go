package tui

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestLogsModelCancelCancelsContext(t *testing.T) {
	pod := &corev1.Pod{}
	m := NewLogsModel(nil, pod)

	if err := m.ctx.Err(); err != nil {
		t.Fatalf("expected fresh context to be uncanceled, got %v", err)
	}

	m.Cancel()

	if err := m.ctx.Err(); err != context.Canceled {
		t.Errorf("expected context.Canceled after Cancel(), got %v", err)
	}
}
