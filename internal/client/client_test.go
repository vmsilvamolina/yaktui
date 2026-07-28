package client

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newTestClient() (*Client, *fake.Clientset) {
	fakeClientset := fake.NewSimpleClientset()
	c := &Client{
		clientset: fakeClientset,
		namespace: "default",
	}
	return c, fakeClientset
}

func newAddonTestClient(gvr schema.GroupVersionResource, listKind string, objects ...runtime.Object) (*Client, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: listKind}, objects...)
	c := &Client{
		dynamicClient: dynClient,
		namespace:     "default",
	}
	return c, dynClient
}

func TestNamespaceGetSet(t *testing.T) {
	c, _ := newTestClient()
	if c.Namespace() != "default" {
		t.Fatalf("expected default namespace, got %q", c.Namespace())
	}
	c.SetNamespace("kube-system")
	if c.Namespace() != "kube-system" {
		t.Fatalf("expected kube-system namespace, got %q", c.Namespace())
	}
}

func TestGetClusterInfo(t *testing.T) {
	c, _ := newTestClient()
	c.clusterInfo = ClusterInfo{Context: "ctx", Cluster: "cluster", Version: "v1.29.0"}
	info := c.GetClusterInfo()
	if info.Context != "ctx" || info.Cluster != "cluster" || info.Version != "v1.29.0" {
		t.Fatalf("unexpected cluster info: %+v", info)
	}
}

func TestExecCmd(t *testing.T) {
	c, _ := newTestClient()
	c.kubeconfig = "/tmp/kubeconfig"

	cmd := c.ExecCmd("mypod", "mycontainer", "myns")
	args := cmd.Args
	joined := strings.Join(args, " ")

	for _, want := range []string{"exec", "-it", "mypod", "-n", "myns", "--kubeconfig", "/tmp/kubeconfig", "-c", "mycontainer", "--", "/bin/sh"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected args to contain %q, got %q", want, joined)
		}
	}
}

func TestExecCmdNoContainerNoKubeconfig(t *testing.T) {
	c, _ := newTestClient()
	cmd := c.ExecCmd("mypod", "", "myns")
	joined := strings.Join(cmd.Args, " ")

	if strings.Contains(joined, "--kubeconfig") {
		t.Errorf("expected no --kubeconfig flag, got %q", joined)
	}
	if strings.Contains(joined, "-c ") {
		t.Errorf("expected no -c flag, got %q", joined)
	}
}

func TestCheckConnection(t *testing.T) {
	c, _ := newTestClient()
	if err := c.CheckConnection(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCheckConnectionError(t *testing.T) {
	c, fakeClientset := newTestClient()
	fakeClientset.PrependReactor("list", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	if err := c.CheckConnection(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListNamespaces(t *testing.T) {
	c, fakeClientset := newTestClient()
	_, err := fakeClientset.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-a"},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	list, err := c.ListNamespaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "ns-a" {
		t.Fatalf("unexpected namespaces: %+v", list)
	}
}

func TestListAndGetPods(t *testing.T) {
	c, fakeClientset := newTestClient()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}}
	otherPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "other"}}
	if _, err := fakeClientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.CoreV1().Pods("other").Create(context.Background(), otherPod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	pods, err := c.ListPods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 1 || pods[0].Name != "pod-a" {
		t.Fatalf("unexpected pods: %+v", pods)
	}

	allPods, err := c.ListAllPods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allPods) != 2 {
		t.Fatalf("expected 2 pods across namespaces, got %d", len(allPods))
	}

	got, err := c.GetPod(context.Background(), "pod-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "pod-a" {
		t.Fatalf("expected pod-a, got %s", got.Name)
	}

	if _, err := c.GetPod(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected error for missing pod")
	}
}

func TestDeletePod(t *testing.T) {
	c, fakeClientset := newTestClient()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}}
	if _, err := fakeClientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := c.DeletePod(context.Background(), "pod-a"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.GetPod(context.Background(), "pod-a"); err == nil {
		t.Fatal("expected pod to be deleted")
	}
}

func TestListAndGetDeployments(t *testing.T) {
	c, fakeClientset := newTestClient()
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "dep-a", Namespace: "default"}}
	otherDep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "dep-b", Namespace: "other"}}
	if _, err := fakeClientset.AppsV1().Deployments("default").Create(context.Background(), dep, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.AppsV1().Deployments("other").Create(context.Background(), otherDep, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	deps, err := c.ListDeployments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Name != "dep-a" {
		t.Fatalf("unexpected deployments: %+v", deps)
	}

	allDeps, err := c.ListAllDeployments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allDeps) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(allDeps))
	}

	got, err := c.GetDeployment(context.Background(), "dep-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "dep-a" {
		t.Fatalf("expected dep-a, got %s", got.Name)
	}
}

func TestListAndGetServices(t *testing.T) {
	c, fakeClientset := newTestClient()
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "default"}}
	otherSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-b", Namespace: "other"}}
	if _, err := fakeClientset.CoreV1().Services("default").Create(context.Background(), svc, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.CoreV1().Services("other").Create(context.Background(), otherSvc, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	svcs, err := c.ListServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 1 || svcs[0].Name != "svc-a" {
		t.Fatalf("unexpected services: %+v", svcs)
	}

	allSvcs, err := c.ListAllServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSvcs) != 2 {
		t.Fatalf("expected 2 services, got %d", len(allSvcs))
	}

	got, err := c.GetService(context.Background(), "svc-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "svc-a" {
		t.Fatalf("expected svc-a, got %s", got.Name)
	}
}

func TestListConfigMaps(t *testing.T) {
	c, fakeClientset := newTestClient()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm-a", Namespace: "default"}}
	otherCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm-b", Namespace: "other"}}
	if _, err := fakeClientset.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.CoreV1().ConfigMaps("other").Create(context.Background(), otherCm, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	cms, err := c.ListConfigMaps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cms) != 1 || cms[0].Name != "cm-a" {
		t.Fatalf("unexpected configmaps: %+v", cms)
	}

	allCms, err := c.ListAllConfigMaps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allCms) != 2 {
		t.Fatalf("expected 2 configmaps, got %d", len(allCms))
	}
}

func TestListSecrets(t *testing.T) {
	c, fakeClientset := newTestClient()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec-a", Namespace: "default"}}
	otherSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec-b", Namespace: "other"}}
	if _, err := fakeClientset.CoreV1().Secrets("default").Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.CoreV1().Secrets("other").Create(context.Background(), otherSecret, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	secs, err := c.ListSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 1 || secs[0].Name != "sec-a" {
		t.Fatalf("unexpected secrets: %+v", secs)
	}

	allSecs, err := c.ListAllSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSecs) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(allSecs))
	}
}

func TestListReplicaSets(t *testing.T) {
	c, fakeClientset := newTestClient()
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "rs-a", Namespace: "default"}}
	if _, err := fakeClientset.AppsV1().ReplicaSets("default").Create(context.Background(), rs, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	rss, err := c.ListReplicaSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rss) != 1 || rss[0].Name != "rs-a" {
		t.Fatalf("unexpected replicasets: %+v", rss)
	}
}

func TestListNodes(t *testing.T) {
	c, fakeClientset := newTestClient()
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
	if _, err := fakeClientset.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	nodes, err := c.ListNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "node-a" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
}

func TestListStatefulSets(t *testing.T) {
	c, fakeClientset := newTestClient()
	ss := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "ss-a", Namespace: "default"}}
	otherSs := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "ss-b", Namespace: "other"}}
	if _, err := fakeClientset.AppsV1().StatefulSets("default").Create(context.Background(), ss, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.AppsV1().StatefulSets("other").Create(context.Background(), otherSs, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	sets, err := c.ListStatefulSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].Name != "ss-a" {
		t.Fatalf("unexpected statefulsets: %+v", sets)
	}

	allSets, err := c.ListAllStatefulSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSets) != 2 {
		t.Fatalf("expected 2 statefulsets, got %d", len(allSets))
	}
}

func TestListDaemonSets(t *testing.T) {
	c, fakeClientset := newTestClient()
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "ds-a", Namespace: "default"}}
	otherDs := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "ds-b", Namespace: "other"}}
	if _, err := fakeClientset.AppsV1().DaemonSets("default").Create(context.Background(), ds, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.AppsV1().DaemonSets("other").Create(context.Background(), otherDs, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	sets, err := c.ListDaemonSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].Name != "ds-a" {
		t.Fatalf("unexpected daemonsets: %+v", sets)
	}

	allSets, err := c.ListAllDaemonSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSets) != 2 {
		t.Fatalf("expected 2 daemonsets, got %d", len(allSets))
	}
}

func TestListEvents(t *testing.T) {
	c, fakeClientset := newTestClient()
	ev := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "ev-a", Namespace: "default"}}
	otherEv := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "ev-b", Namespace: "other"}}
	if _, err := fakeClientset.CoreV1().Events("default").Create(context.Background(), ev, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fakeClientset.CoreV1().Events("other").Create(context.Background(), otherEv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	evs, err := c.ListEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Name != "ev-a" {
		t.Fatalf("unexpected events: %+v", evs)
	}

	allEvs, err := c.ListAllEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(allEvs) != 2 {
		t.Fatalf("expected 2 events, got %d", len(allEvs))
	}
}

func TestGetPodLogs(t *testing.T) {
	c, fakeClientset := newTestClient()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}}
	if _, err := fakeClientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	logs, err := c.GetPodLogs(context.Background(), "pod-a", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if logs == "" {
		t.Fatal("expected non-empty logs")
	}
}

func TestStreamPodLogs(t *testing.T) {
	c, fakeClientset := newTestClient()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}}
	if _, err := fakeClientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	stream, err := c.StreamPodLogs(context.Background(), "pod-a", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty stream")
	}
}

func TestFetchServerVersion(t *testing.T) {
	c, fakeClientset := newTestClient()
	fakeDiscovery, ok := fakeClientset.Discovery().(*discoveryfake.FakeDiscovery)
	if !ok {
		t.Fatal("expected FakeDiscovery")
	}
	fakeDiscovery.FakedServerVersion = &version.Info{GitVersion: "v1.29.0"}

	v, err := c.FetchServerVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "v1.29.0" {
		t.Fatalf("expected v1.29.0, got %q", v)
	}
	if c.GetClusterInfo().Version != "v1.29.0" {
		t.Fatalf("expected cached version v1.29.0, got %q", c.GetClusterInfo().Version)
	}
}

func TestDetectAPIGroups(t *testing.T) {
	c, fakeClientset := newTestClient()
	fakeDiscovery, ok := fakeClientset.Discovery().(*discoveryfake.FakeDiscovery)
	if !ok {
		t.Fatal("expected FakeDiscovery")
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{GroupVersion: "kyverno.io/v1"},
		{GroupVersion: "v1"},
	}

	groups, err := c.DetectAPIGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !groups["kyverno.io"] {
		t.Fatalf("expected kyverno.io to be detected, got %+v", groups)
	}
	if !groups[""] {
		t.Fatalf("expected core group (empty name) to be detected, got %+v", groups)
	}
}

func TestDetectAPIGroupsError(t *testing.T) {
	c, fakeClientset := newTestClient()
	fakeDiscovery, ok := fakeClientset.Discovery().(*discoveryfake.FakeDiscovery)
	if !ok {
		t.Fatal("expected FakeDiscovery")
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{GroupVersion: "not a valid group version///"},
	}

	if _, err := c.DetectAPIGroups(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func kyvernoGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}
}

func newUnstructuredPolicy(kind, name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kyverno.io/v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
	if namespace != "" {
		obj.Object["metadata"].(map[string]interface{})["namespace"] = namespace
	}
	return obj
}

func TestListAddonResourceClusterScoped(t *testing.T) {
	gvr := kyvernoGVR()
	c, _ := newAddonTestClient(gvr, "ClusterPolicyList", newUnstructuredPolicy("ClusterPolicy", "policy-a", ""))

	list, err := c.ListAddonResource(context.Background(), gvr, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].GetName() != "policy-a" {
		t.Fatalf("unexpected items: %+v", list.Items)
	}
}

func TestListAddonResourceNamespaced(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policies"}
	c, _ := newAddonTestClient(gvr, "PolicyList",
		newUnstructuredPolicy("Policy", "pol-a", "default"),
		newUnstructuredPolicy("Policy", "pol-b", "other"),
	)

	list, err := c.ListAddonResource(context.Background(), gvr, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].GetName() != "pol-a" {
		t.Fatalf("unexpected items: %+v", list.Items)
	}
}

func TestListAllAddonResource(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policies"}
	c, _ := newAddonTestClient(gvr, "PolicyList",
		newUnstructuredPolicy("Policy", "pol-a", "default"),
		newUnstructuredPolicy("Policy", "pol-b", "other"),
	)

	list, err := c.ListAllAddonResource(context.Background(), gvr)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 items across namespaces, got %d", len(list.Items))
	}
}

const testKubeconfig = `
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
    namespace: test-ns
current-context: test-context
users: []
`

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListContexts(t *testing.T) {
	path := writeTestKubeconfig(t)

	names, current, err := ListContexts(path)
	if err != nil {
		t.Fatal(err)
	}
	if current != "test-context" {
		t.Fatalf("expected test-context, got %q", current)
	}
	if len(names) != 1 || names[0] != "test-context" {
		t.Fatalf("unexpected contexts: %+v", names)
	}
}

func TestNew(t *testing.T) {
	path := writeTestKubeconfig(t)

	c, err := New(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Namespace() != "default" {
		t.Fatalf("expected default namespace when empty, got %q", c.Namespace())
	}
	info := c.GetClusterInfo()
	if info.Context != "test-context" {
		t.Fatalf("expected test-context, got %q", info.Context)
	}
	if info.Cluster != "test-cluster" {
		t.Fatalf("expected test-cluster, got %q", info.Cluster)
	}
}

func TestNewWithExplicitNamespace(t *testing.T) {
	path := writeTestKubeconfig(t)

	c, err := New(path, "custom-ns")
	if err != nil {
		t.Fatal(err)
	}
	if c.Namespace() != "custom-ns" {
		t.Fatalf("expected custom-ns, got %q", c.Namespace())
	}
}

func TestNewWithContext(t *testing.T) {
	path := writeTestKubeconfig(t)

	c, err := NewWithContext(path, "test-context", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Namespace() != "default" {
		t.Fatalf("expected default namespace, got %q", c.Namespace())
	}
	info := c.GetClusterInfo()
	if info.Context != "test-context" || info.Cluster != "test-cluster" {
		t.Fatalf("unexpected cluster info: %+v", info)
	}
}

func TestNewInvalidKubeconfig(t *testing.T) {
	if _, err := New("/nonexistent/path/kubeconfig", ""); err == nil {
		t.Fatal("expected error for nonexistent kubeconfig")
	}
}
