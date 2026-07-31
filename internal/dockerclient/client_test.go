package dockerclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/pkg/stdcopy"
)

// fakeAPIClient is a hand-written stand-in for the Docker SDK client,
// implementing apiClient. The SDK ships no fake comparable to k8s'
// fake.Clientset, so tests configure behavior via the exported fields.
type fakeAPIClient struct {
	containers    []container.Summary
	containersErr error

	images    []image.Summary
	imagesErr error

	volumes    volume.ListResponse
	volumesErr error

	networks    []network.Summary
	networksErr error

	startErr   error
	stopErr    error
	restartErr error
	removeErr  error

	inspectResp container.InspectResponse
	inspectErr  error

	logs    string
	logsErr error

	info    system.Info
	infoErr error

	gotStartID   string
	gotStopID    string
	gotRestartID string
	gotRemoveID  string
	gotRemove    container.RemoveOptions
	gotLogsID    string
	gotLogsOpts  container.LogsOptions
}

func (f *fakeAPIClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	return f.containers, f.containersErr
}

func (f *fakeAPIClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return f.images, f.imagesErr
}

func (f *fakeAPIClient) VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error) {
	return f.volumes, f.volumesErr
}

func (f *fakeAPIClient) NetworkList(ctx context.Context, options network.ListOptions) ([]network.Summary, error) {
	return f.networks, f.networksErr
}

func (f *fakeAPIClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	f.gotStartID = containerID
	return f.startErr
}

func (f *fakeAPIClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	f.gotStopID = containerID
	return f.stopErr
}

func (f *fakeAPIClient) ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error {
	f.gotRestartID = containerID
	return f.restartErr
}

func (f *fakeAPIClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	f.gotRemoveID = containerID
	f.gotRemove = options
	return f.removeErr
}

func (f *fakeAPIClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return f.inspectResp, f.inspectErr
}

func (f *fakeAPIClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	f.gotLogsID = containerID
	f.gotLogsOpts = options
	if f.logsErr != nil {
		return nil, f.logsErr
	}
	return io.NopCloser(strings.NewReader(f.logs)), nil
}

func (f *fakeAPIClient) Info(ctx context.Context) (system.Info, error) {
	return f.info, f.infoErr
}

func newTestClient(fake *fakeAPIClient) *Client {
	return &Client{cli: fake}
}

func TestListContainers(t *testing.T) {
	fake := &fakeAPIClient{containers: []container.Summary{{ID: "c1"}, {ID: "c2"}}}
	c := newTestClient(fake)

	got, err := c.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(got))
	}
}

func TestListContainersError(t *testing.T) {
	fake := &fakeAPIClient{containersErr: errors.New("daemon down")}
	c := newTestClient(fake)

	if _, err := c.ListContainers(context.Background(), false); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListImages(t *testing.T) {
	fake := &fakeAPIClient{images: []image.Summary{{ID: "img1"}}}
	c := newTestClient(fake)

	got, err := c.ListImages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "img1" {
		t.Fatalf("unexpected images: %+v", got)
	}
}

func TestListVolumes(t *testing.T) {
	fake := &fakeAPIClient{volumes: volume.ListResponse{Volumes: []*volume.Volume{{Name: "vol-a"}}}}
	c := newTestClient(fake)

	got, err := c.ListVolumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "vol-a" {
		t.Fatalf("unexpected volumes: %+v", got)
	}
}

func TestListVolumesError(t *testing.T) {
	fake := &fakeAPIClient{volumesErr: errors.New("daemon down")}
	c := newTestClient(fake)

	if _, err := c.ListVolumes(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListNetworks(t *testing.T) {
	fake := &fakeAPIClient{networks: []network.Summary{{ID: "net-a"}}}
	c := newTestClient(fake)

	got, err := c.ListNetworks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "net-a" {
		t.Fatalf("unexpected networks: %+v", got)
	}
}

func TestStartContainer(t *testing.T) {
	fake := &fakeAPIClient{}
	c := newTestClient(fake)

	if err := c.StartContainer(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if fake.gotStartID != "c1" {
		t.Fatalf("expected start called with c1, got %q", fake.gotStartID)
	}
}

func TestStopContainer(t *testing.T) {
	fake := &fakeAPIClient{}
	c := newTestClient(fake)

	if err := c.StopContainer(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if fake.gotStopID != "c1" {
		t.Fatalf("expected stop called with c1, got %q", fake.gotStopID)
	}
}

func TestRestartContainer(t *testing.T) {
	fake := &fakeAPIClient{}
	c := newTestClient(fake)

	if err := c.RestartContainer(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if fake.gotRestartID != "c1" {
		t.Fatalf("expected restart called with c1, got %q", fake.gotRestartID)
	}
}

func TestRemoveContainer(t *testing.T) {
	fake := &fakeAPIClient{}
	c := newTestClient(fake)

	if err := c.RemoveContainer(context.Background(), "c1", true); err != nil {
		t.Fatal(err)
	}
	if fake.gotRemoveID != "c1" {
		t.Fatalf("expected remove called with c1, got %q", fake.gotRemoveID)
	}
	if !fake.gotRemove.Force {
		t.Fatal("expected Force to be true")
	}
}

func TestRemoveContainerError(t *testing.T) {
	fake := &fakeAPIClient{removeErr: errors.New("in use")}
	c := newTestClient(fake)

	if err := c.RemoveContainer(context.Background(), "c1", false); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInfo(t *testing.T) {
	fake := &fakeAPIClient{info: system.Info{ServerVersion: "24.0.0", Containers: 3}}
	c := newTestClient(fake)

	got, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.ServerVersion != "24.0.0" || got.Containers != 3 {
		t.Fatalf("unexpected info: %+v", got)
	}
}

func TestGetContainerLogsNonTTY(t *testing.T) {
	var framed bytes.Buffer
	stdoutW := stdcopy.NewStdWriter(&framed, stdcopy.Stdout)
	if _, err := stdoutW.Write([]byte("hello stdout\n")); err != nil {
		t.Fatal(err)
	}
	stderrW := stdcopy.NewStdWriter(&framed, stdcopy.Stderr)
	if _, err := stderrW.Write([]byte("hello stderr\n")); err != nil {
		t.Fatal(err)
	}

	fake := &fakeAPIClient{
		inspectResp: container.InspectResponse{Config: &container.Config{Tty: false}},
		logs:        framed.String(),
	}
	c := newTestClient(fake)

	got, err := c.GetContainerLogs(context.Background(), "c1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "hello stdout") || !strings.Contains(got, "hello stderr") {
		t.Fatalf("expected demuxed output to contain both lines, got %q", got)
	}
	if fake.gotLogsID != "c1" {
		t.Fatalf("expected logs requested for c1, got %q", fake.gotLogsID)
	}
	if fake.gotLogsOpts.Tail != "100" {
		t.Fatalf("expected tail 100, got %q", fake.gotLogsOpts.Tail)
	}
}

func TestGetContainerLogsTTY(t *testing.T) {
	fake := &fakeAPIClient{
		inspectResp: container.InspectResponse{Config: &container.Config{Tty: true}},
		logs:        "raw tty output\n",
	}
	c := newTestClient(fake)

	got, err := c.GetContainerLogs(context.Background(), "c1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if got != "raw tty output\n" {
		t.Fatalf("expected raw passthrough, got %q", got)
	}
}

func TestGetContainerLogsInspectError(t *testing.T) {
	fake := &fakeAPIClient{inspectErr: errors.New("no such container")}
	c := newTestClient(fake)

	if _, err := c.GetContainerLogs(context.Background(), "c1", 10); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetContainerLogsStreamError(t *testing.T) {
	fake := &fakeAPIClient{
		inspectResp: container.InspectResponse{Config: &container.Config{Tty: false}},
		logsErr:     errors.New("logs unavailable"),
	}
	c := newTestClient(fake)

	if _, err := c.GetContainerLogs(context.Background(), "c1", 10); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecCmd(t *testing.T) {
	c := newTestClient(&fakeAPIClient{})

	cmd := c.ExecCmd("c1")
	joined := strings.Join(cmd.Args, " ")
	for _, want := range []string{"docker", "exec", "-it", "c1", "/bin/sh"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected args to contain %q, got %q", want, joined)
		}
	}
}
