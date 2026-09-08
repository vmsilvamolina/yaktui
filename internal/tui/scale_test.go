package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScaleKeyBindings(t *testing.T) {
	for _, k := range []string{"+", "="} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if !key.Matches(msg, DefaultKeyMap.ScaleUp) {
			t.Errorf("expected ScaleUp binding to match %q", k)
		}
	}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")}
	if !key.Matches(msg, DefaultKeyMap.ScaleDown) {
		t.Error("expected ScaleDown binding to match '-'")
	}
}

func TestDeploymentReplicas(t *testing.T) {
	if got := deploymentReplicas(&appsv1.Deployment{}); got != 0 {
		t.Errorf("nil Spec.Replicas should read as 0, got %d", got)
	}
	three := int32(3)
	dep := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: &three}}
	if got := deploymentReplicas(dep); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestGetSelectedDeploymentAfterMsg(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewDeploymentsModel(c)
	m.SetSize(120, 30)

	two := int32(2)
	newModel, _ := m.Update(DeploymentsMsg{Deployments: []appsv1.Deployment{
		{ObjectMeta: metav1.ObjectMeta{Name: "dep-a", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &two}},
	}})
	m = newModel.(*DeploymentsModel)

	sel := m.GetSelectedDeployment()
	if sel == nil || sel.Name != "dep-a" {
		t.Fatalf("expected dep-a selected, got %+v", sel)
	}
	if deploymentReplicas(sel) != 2 {
		t.Fatalf("expected 2 replicas, got %d", deploymentReplicas(sel))
	}
}

func TestScaleDeploymentResultMsgStatus(t *testing.T) {
	c := newAddonTestClient(t, "default")
	m := NewModel(c, "/tmp/kubeconfig", nil)

	newModel, _ := m.Update(ScaleDeploymentResultMsg{Name: "dep-a", Replicas: 4})
	if got := newModel.(Model).statusMessage; got != "scaled dep-a to 4 replicas" {
		t.Fatalf("unexpected status on success: %q", got)
	}
}
