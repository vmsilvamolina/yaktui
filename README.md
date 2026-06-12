# YAKTUI

Yet Another Kubernetes TUI — a minimal terminal UI for Kubernetes, inspired by [k9s](https://github.com/derailed/k9s).

Built with [Bubbletea](https://github.com/charmbracelet/bubbletea).

## Features

- **Multiple resources** — Pods, Deployments, StatefulSets, DaemonSets, Services, ConfigMaps, Secrets, Namespaces, Nodes, Events
- **Cluster info bar** — current context, cluster, namespace, and k8s version always visible
- **Pod relations** — visualize relationships with Deployments, ReplicaSets, and Services
- **Log streaming** — real-time logs with follow mode
- **Pod shell exec** — open an interactive shell inside any container
- **YAML describe** — full YAML view for any resource (secrets redacted)
- **Search / filter** — press `/` to filter by name in any resource list
- **Namespace toggle** — switch between current and all namespaces instantly
- **Delete with confirmation** — modal confirmation before deleting pods
- **Auto-refresh** — active resource refreshes every 30 seconds
- **Vim navigation** — `hjkl` throughout

## Installation

### Homebrew (recommended)

```bash
brew tap vmsilvamolina/yaktui
brew install yaktui
```

### Go install

```bash
go install github.com/vmsilvamolina/yaktui@latest
```

### From source

```bash
git clone https://github.com/vmsilvamolina/yaktui.git
cd yaktui
make build
./bin/yaktui
```

## Usage

```bash
# Start with default kubeconfig (~/.kube/config)
yaktui

# Use a specific kubeconfig
yaktui --kubeconfig /path/to/kubeconfig

# Start in a specific namespace
yaktui -n kube-system

# Show version
yaktui version
```

## Keyboard Shortcuts

### Global

| Key | Action |
|-----|--------|
| `tab` | Switch between sidebar and content panel |
| `a` | Toggle all namespaces |
| `/` | Search / filter by name |
| `ctrl+c` | Quit |

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` / `esc` | Go back / focus sidebar |
| `→` / `l` / `enter` | Select / enter resource |

### List view (any resource)

| Key | Action |
|-----|--------|
| `d` | YAML describe |
| `a` | Toggle all namespaces |
| `/` | Search (type to filter, `esc` to clear) |

### Pods

| Key | Action |
|-----|--------|
| `enter` | View pod relations |
| `l` | Stream logs |
| `s` | Open shell in container |
| `d` | YAML describe |
| `del` / `backspace` | Delete pod (with confirmation) |

### Logs

| Key | Action |
|-----|--------|
| `f` | Toggle follow mode |
| `g` / `G` | Scroll to top / bottom |
| `esc` | Go back |

### YAML view

| Key | Action |
|-----|--------|
| `↑↓` / `jk` | Scroll |
| `g` / `G` | Top / bottom |
| `esc` | Go back |

### Namespaces

| Key | Action |
|-----|--------|
| `enter` | Switch to selected namespace |

## Requirements

- A running Kubernetes cluster
- A valid kubeconfig (`~/.kube/config` by default)

## Tech stack

- [Bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling
- [client-go](https://github.com/kubernetes/client-go) — Kubernetes client

## License

MIT
