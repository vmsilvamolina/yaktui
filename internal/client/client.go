package client

import (
	"context"
	"io"
	"os/exec"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterInfo holds metadata about the connected cluster
type ClusterInfo struct {
	Context string
	Cluster string
	Version string
}

// Client wraps the Kubernetes clientset
type Client struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	kubeconfig    string
	clusterInfo   ClusterInfo
}

// New creates a new Kubernetes client
func New(kubeconfig, namespace string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = "default"
	}

	info := ClusterInfo{}
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{},
	).RawConfig()
	if err == nil {
		info.Context = rawConfig.CurrentContext
		if ctx, ok := rawConfig.Contexts[rawConfig.CurrentContext]; ok && ctx != nil {
			info.Cluster = ctx.Cluster
		}
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		kubeconfig:    kubeconfig,
		clusterInfo:   info,
	}, nil
}

// GetClusterInfo returns cached context/cluster info
func (c *Client) GetClusterInfo() ClusterInfo {
	return c.clusterInfo
}

// FetchServerVersion queries the API server version and caches it
func (c *Client) FetchServerVersion(ctx context.Context) (string, error) {
	sv, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	c.clusterInfo.Version = sv.GitVersion
	return sv.GitVersion, nil
}

// ExecCmd returns a command to open an interactive shell in a pod container.
// The caller is responsible for connecting stdin/stdout/stderr before running it.
func (c *Client) ExecCmd(podName, containerName, namespace string) *exec.Cmd {
	args := []string{"exec", "-it", podName, "-n", namespace}
	if c.kubeconfig != "" {
		args = append(args, "--kubeconfig", c.kubeconfig)
	}
	if containerName != "" {
		args = append(args, "-c", containerName)
	}
	args = append(args, "--", "/bin/sh")
	return exec.Command("kubectl", args...)
}

// CheckConnection verifies the connection to the Kubernetes cluster
func (c *Client) CheckConnection(ctx context.Context) error {
	_, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}

// Namespace returns current namespace
func (c *Client) Namespace() string {
	return c.namespace
}

// SetNamespace changes the current namespace
func (c *Client) SetNamespace(ns string) {
	c.namespace = ns
}

// ListNamespaces returns all namespaces
func (c *Client) ListNamespaces(ctx context.Context) ([]corev1.Namespace, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListPods returns pods in the current namespace
func (c *Client) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	list, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllPods returns pods in all namespaces
func (c *Client) ListAllPods(ctx context.Context) ([]corev1.Pod, error) {
	list, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPod returns a specific pod
func (c *Client) GetPod(ctx context.Context, name string) (*corev1.Pod, error) {
	return c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetPodLogs returns logs for a pod container
func (c *Client) GetPodLogs(ctx context.Context, podName, containerName string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	}

	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = stream.Close() }()

	logs, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}

	return string(logs), nil
}

// StreamPodLogs streams logs for a pod container
func (c *Client) StreamPodLogs(ctx context.Context, podName, containerName string, tailLines int64) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
		Follow:    true,
	}

	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, opts)
	return req.Stream(ctx)
}

// ListServices returns services in the current namespace
func (c *Client) ListServices(ctx context.Context) ([]corev1.Service, error) {
	list, err := c.clientset.CoreV1().Services(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// DeletePod deletes a pod
func (c *Client) DeletePod(ctx context.Context, name string) error {
	return c.clientset.CoreV1().Pods(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListDeployments returns deployments in the current namespace
func (c *Client) ListDeployments(ctx context.Context) ([]appsv1.Deployment, error) {
	list, err := c.clientset.AppsV1().Deployments(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllDeployments returns deployments in all namespaces
func (c *Client) ListAllDeployments(ctx context.Context) ([]appsv1.Deployment, error) {
	list, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllServices returns services in all namespaces
func (c *Client) ListAllServices(ctx context.Context) ([]corev1.Service, error) {
	list, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListConfigMaps returns configmaps in the current namespace
func (c *Client) ListConfigMaps(ctx context.Context) ([]corev1.ConfigMap, error) {
	list, err := c.clientset.CoreV1().ConfigMaps(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllConfigMaps returns configmaps in all namespaces
func (c *Client) ListAllConfigMaps(ctx context.Context) ([]corev1.ConfigMap, error) {
	list, err := c.clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListSecrets returns secrets in the current namespace
func (c *Client) ListSecrets(ctx context.Context) ([]corev1.Secret, error) {
	list, err := c.clientset.CoreV1().Secrets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllSecrets returns secrets in all namespaces
func (c *Client) ListAllSecrets(ctx context.Context) ([]corev1.Secret, error) {
	list, err := c.clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListReplicaSets returns replicasets in the current namespace
func (c *Client) ListReplicaSets(ctx context.Context) ([]appsv1.ReplicaSet, error) {
	list, err := c.clientset.AppsV1().ReplicaSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetDeployment returns a specific deployment
func (c *Client) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetService returns a specific service
func (c *Client) GetService(ctx context.Context, name string) (*corev1.Service, error) {
	return c.clientset.CoreV1().Services(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListNodes returns all nodes in the cluster
func (c *Client) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	list, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListStatefulSets returns statefulsets in the current namespace
func (c *Client) ListStatefulSets(ctx context.Context) ([]appsv1.StatefulSet, error) {
	list, err := c.clientset.AppsV1().StatefulSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllStatefulSets returns statefulsets in all namespaces
func (c *Client) ListAllStatefulSets(ctx context.Context) ([]appsv1.StatefulSet, error) {
	list, err := c.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListDaemonSets returns daemonsets in the current namespace
func (c *Client) ListDaemonSets(ctx context.Context) ([]appsv1.DaemonSet, error) {
	list, err := c.clientset.AppsV1().DaemonSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllDaemonSets returns daemonsets in all namespaces
func (c *Client) ListAllDaemonSets(ctx context.Context) ([]appsv1.DaemonSet, error) {
	list, err := c.clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListEvents returns events in the current namespace
func (c *Client) ListEvents(ctx context.Context) ([]corev1.Event, error) {
	list, err := c.clientset.CoreV1().Events(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListAllEvents returns events in all namespaces
func (c *Client) ListAllEvents(ctx context.Context) ([]corev1.Event, error) {
	list, err := c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// DetectAPIGroups returns the set of API group names registered on the
// cluster, used to auto-detect whether an addon's CRD is installed.
func (c *Client) DetectAPIGroups(ctx context.Context) (map[string]bool, error) {
	groups, err := c.clientset.Discovery().ServerGroups()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(groups.Groups))
	for _, g := range groups.Groups {
		set[g.Name] = true
	}
	return set, nil
}

// ListAddonResource returns items of an addon's CRD in the current namespace
// (or cluster-wide if namespaced is false).
func (c *Client) ListAddonResource(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool) (*unstructured.UnstructuredList, error) {
	ri := c.dynamicClient.Resource(gvr)
	if namespaced {
		return ri.Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	}
	return ri.List(ctx, metav1.ListOptions{})
}

// ListAllAddonResource returns items of an addon's CRD across all namespaces.
func (c *Client) ListAllAddonResource(ctx context.Context, gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	return c.dynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
}

// ListContexts returns sorted context names from the kubeconfig and the active context.
func ListContexts(kubeconfig string) ([]string, string, error) {
	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, rawConfig.CurrentContext, nil
}

// NewWithContext creates a Client using a specific kubeconfig context.
func NewWithContext(kubeconfig, contextName, namespace string) (*Client, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		overrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = "default"
	}

	info := ClusterInfo{Context: contextName}
	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		overrides,
	).RawConfig()
	if err == nil {
		if ctx, ok := rawConfig.Contexts[contextName]; ok && ctx != nil {
			info.Cluster = ctx.Cluster
		}
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		kubeconfig:    kubeconfig,
		clusterInfo:   info,
	}, nil
}
