package addons

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefinitionGVR(t *testing.T) {
	d := Definition{Group: "kyverno.io", Version: "v1"}
	r := Resource{Kind: "ClusterPolicy", Resource: "clusterpolicies", ClusterScoped: true}
	want := schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}
	if got := d.GVR(r); got != want {
		t.Fatalf("GVR() = %+v, want %+v", got, want)
	}
}

func TestDefinitionHasNamespacedResource(t *testing.T) {
	clusterOnly := Definition{Resources: []Resource{{ClusterScoped: true}}}
	if clusterOnly.HasNamespacedResource() {
		t.Error("expected false for cluster-scoped-only definition")
	}

	mixed := Definition{Resources: []Resource{{ClusterScoped: true}, {ClusterScoped: false}}}
	if !mixed.HasNamespacedResource() {
		t.Error("expected true for mixed definition")
	}

	namespacedOnly := Definition{Resources: []Resource{{ClusterScoped: false}}}
	if !namespacedOnly.HasNamespacedResource() {
		t.Error("expected true for namespaced-only definition")
	}
}

func TestMergeOverridesByName(t *testing.T) {
	base := []Definition{
		{Name: "a", DisplayName: "A"},
		{Name: "b", DisplayName: "B"},
	}
	overrides := []Definition{
		{Name: "a", DisplayName: "A-overridden"},
	}

	got := Merge(base, overrides)
	want := []Definition{
		{Name: "a", DisplayName: "A-overridden"},
		{Name: "b", DisplayName: "B"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge() = %+v, want %+v", got, want)
	}
}

func TestMergeAppendsNew(t *testing.T) {
	base := []Definition{{Name: "a", DisplayName: "A"}}
	overrides := []Definition{{Name: "c", DisplayName: "C"}}

	got := Merge(base, overrides)
	want := []Definition{
		{Name: "a", DisplayName: "A"},
		{Name: "c", DisplayName: "C"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge() = %+v, want %+v", got, want)
	}
}

func TestMergeDoesNotMutateBase(t *testing.T) {
	base := []Definition{{Name: "a", DisplayName: "A"}}
	_ = Merge(base, []Definition{{Name: "a", DisplayName: "A-overridden"}})

	if base[0].DisplayName != "A" {
		t.Fatalf("Merge mutated base slice: %+v", base)
	}
}

func TestActivateDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		enabled    string
		detected   bool
		wantActive bool
	}{
		{"false disabled, detected", "false", true, false},
		{"false disabled, not detected", "false", false, false},
		{"true forced, detected", "true", true, true},
		{"true forced, not detected", "true", false, true},
		{"auto, detected", "auto", true, true},
		{"auto, not detected", "auto", false, false},
		{"empty (defaults to auto), detected", "", true, true},
		{"empty (defaults to auto), not detected", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := []Definition{{Name: "x", Group: "example.io", Enabled: tt.enabled}}
			detected := map[string]bool{"example.io": tt.detected}

			active := ActivateDefinitions(defs, detected)
			gotActive := len(active) == 1
			if gotActive != tt.wantActive {
				t.Fatalf("ActivateDefinitions() active=%v, want %v", gotActive, tt.wantActive)
			}
		})
	}
}

func TestExtractColumnNested(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"background": true,
			"nested": map[string]interface{}{
				"value": "deep",
			},
		},
		"metadata": map[string]interface{}{
			"name": "my-policy",
		},
	}

	tests := []struct {
		path string
		want string
	}{
		{"metadata.name", "my-policy"},
		{"spec.background", "true"},
		{"spec.nested.value", "deep"},
		{"spec.missing", ""},
		{"totally.missing.path", ""},
	}

	for _, tt := range tests {
		if got := ExtractColumn(obj, tt.path); got != tt.want {
			t.Errorf("ExtractColumn(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractColumnNonMapIntermediate(t *testing.T) {
	obj := map[string]interface{}{
		"spec": "not-a-map",
	}
	if got := ExtractColumn(obj, "spec.background"); got != "" {
		t.Errorf("ExtractColumn() = %q, want empty string", got)
	}
}

func TestExtractColumnNilValue(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"background": nil,
		},
	}
	if got := ExtractColumn(obj, "spec.background"); got != "" {
		t.Errorf("ExtractColumn() = %q, want empty string", got)
	}
}

func TestExtractColumnNumericTypes(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(3),
			"ratio":    1.5,
		},
	}
	if got := ExtractColumn(obj, "spec.replicas"); got != "3" {
		t.Errorf("ExtractColumn() = %q, want %q", got, "3")
	}
	if got := ExtractColumn(obj, "spec.ratio"); got != "1.5" {
		t.Errorf("ExtractColumn() = %q, want %q", got, "1.5")
	}
}
