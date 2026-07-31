// Package dockerclient wraps the Docker SDK client, exposing the subset of
// operations the Docker-mode TUI views need.
package dockerclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// apiClient is the subset of the Docker SDK client this package uses.
// Declared here (rather than depending on the SDK's much larger APIClient
// interface) so tests can supply a hand-written fake without pulling in the
// whole SDK surface.
type apiClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error)
	NetworkList(ctx context.Context, options network.ListOptions) ([]network.Summary, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	Info(ctx context.Context) (system.Info, error)
}

// Client wraps the Docker SDK client.
type Client struct {
	cli apiClient
}

// New connects to the local Docker daemon using the same environment-based
// discovery as the `docker` CLI (DOCKER_HOST, DOCKER_CERT_PATH,
// DOCKER_TLS_VERIFY, or the default unix socket), and pings the daemon once
// to confirm it's reachable. Callers should invoke this lazily (only when
// the user switches into Docker mode), not at startup.
func New() (*Client, error) {
	opts := []dockerclient.Opt{}
	if host := contextHost(); host != "" {
		opts = append(opts, dockerclient.WithHost(host))
	}
	opts = append(opts, dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())

	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker daemon unreachable: %w", err)
	}

	return &Client{cli: cli}, nil
}

// contextHost resolves the Docker daemon endpoint from the active `docker`
// CLI context (e.g. Colima, Docker Desktop, Rancher Desktop), the way the
// real `docker` CLI does. It's consulted only when DOCKER_HOST isn't set —
// the SDK's own FromEnv option always wins when that env var is present,
// matching the CLI's own precedence. Returns "" (falling back to the SDK's
// default socket) if DOCKER_HOST is set, the `docker` binary isn't
// available, or the active context can't be resolved.
func contextHost() string {
	if os.Getenv("DOCKER_HOST") != "" {
		return ""
	}
	out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ListContainers returns containers, including stopped ones when all is true.
func (c *Client) ListContainers(ctx context.Context, all bool) ([]container.Summary, error) {
	return c.cli.ContainerList(ctx, container.ListOptions{All: all})
}

// ListImages returns images known to the daemon.
func (c *Client) ListImages(ctx context.Context) ([]image.Summary, error) {
	return c.cli.ImageList(ctx, image.ListOptions{})
}

// ListVolumes returns volumes known to the daemon.
func (c *Client) ListVolumes(ctx context.Context) ([]*volume.Volume, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	return resp.Volumes, nil
}

// ListNetworks returns networks known to the daemon.
func (c *Client) ListNetworks(ctx context.Context) ([]network.Summary, error) {
	return c.cli.NetworkList(ctx, network.ListOptions{})
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// StopContainer stops a running container.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStop(ctx, id, container.StopOptions{})
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{})
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

// GetContainerLogs returns a bounded, non-following tail of a container's
// combined stdout/stderr logs.
//
// Docker multiplexes a non-TTY container's log stream with an 8-byte frame
// header per chunk, so the raw stream must be demuxed with stdcopy.StdCopy
// rather than read directly, or the output contains binary framing bytes.
// A container started with a TTY, however, gets a single raw stream with no
// framing at all — demuxing that fails ("Unrecognized input header"), so
// the container's TTY setting must be checked first.
func (c *Client) GetContainerLogs(ctx context.Context, id string, tailLines int64) (string, error) {
	inspect, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}

	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       strconv.FormatInt(tailLines, 10),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()

	if inspect.Config != nil && inspect.Config.Tty {
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Info returns daemon-wide stats (version, container/image counts) used for
// the Docker-mode header.
func (c *Client) Info(ctx context.Context) (system.Info, error) {
	return c.cli.Info(ctx)
}

// ExecCmd returns a command to open an interactive shell in a container,
// shelling out to the real `docker` binary — mirrors client.Client.ExecCmd's
// kubectl-delegation pattern. The Docker SDK's own exec API returns a raw
// hijacked stream that the caller would have to pump manually into the
// terminal; delegating to `docker exec -it` avoids that complexity entirely,
// exactly as the Kubernetes side delegates to `kubectl exec` rather than
// using client-go's attach/exec API directly.
func (c *Client) ExecCmd(containerID string) *exec.Cmd {
	return exec.Command("docker", "exec", "-it", containerID, "/bin/sh")
}
