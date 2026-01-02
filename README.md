# YAKTUI

Yet Another Kubernetes TUI - A minimal terminal UI for Kubernetes, inspired by [k9s](https://github.com/derailed/k9s).

Built with [Bubbletea](https://github.com/charmbracelet/bubbletea) 🧋

```
  YAKTUI - Yet Another Kubernetes TUI
```

## Features

- 📋 **Multiple Resources** - View Pods, Deployments, Services, ConfigMaps, Secrets, Namespaces
- 🔗 **Pod Relations** - Visualize pod relationships with Deployments, ReplicaSets, Services
- 📜 **Stream Logs** - Watch container logs in real-time with follow mode
- 🏷️ **Namespace Navigation** - Switch between namespaces easily
- ⌨️ **Vim-like Navigation** - Use j/k for navigation
- 🎨 **Modern UI** - Built with Lip Gloss for beautiful styling

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/vmsilvamolina/yaktui.git
cd yaktui

# Build
make build

# Run
./bin/yaktui
```

### Go Install

```bash
go install github.com/vmsilvamolina/yaktui@latest
```

## Usage

```bash
# Start with default kubeconfig (~/.kube/config)
yaktui

# Use specific kubeconfig
yaktui --kubeconfig /path/to/kubeconfig

# Start in specific namespace
yaktui -n kube-system

# Show version
yaktui version
```

## Keyboard Shortcuts

### Global

| Key | Action |
|-----|--------|
| `tab` | Switch between sidebar and content |
| `ctrl+c` | Quit |
| `a` | Toggle all namespaces |

### Navigation

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `←/h` | Move left / Go to sidebar |
| `→/l` | Move right / Select resource |
| `enter` | Select / View relations (pods) |
| `esc` | Go back |

### Pods View

| Key | Action |
|-----|--------|
| `enter` | View pod relations |
| `l` | View logs |
| `a` | Toggle all namespaces |

### Logs View

| Key | Action |
|-----|--------|
| `esc` | Go back |
| `g` | Scroll to top |
| `G` | Scroll to bottom |
| `f` | Toggle follow mode |

### Relations View

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate resources |
| `enter/l` | View logs (when pod selected) |
| `esc` | Go back |

## Project Structure

```
yaktui/
├── cmd/
│   └── root.go              # CLI commands
├── internal/
│   ├── client/
│   │   └── client.go        # Kubernetes client wrapper
│   └── tui/
│       ├── model.go         # Main Bubbletea model
│       ├── styles.go        # Lip Gloss styles
│       ├── keys.go          # Key bindings
│       ├── pods.go          # Pods view
│       ├── deployments.go   # Deployments view
│       ├── services.go      # Services view
│       ├── configmaps.go    # ConfigMaps view
│       ├── secrets.go       # Secrets view
│       ├── namespaces.go    # Namespaces view
│       ├── relations.go     # Pod relations view
│       └── logs.go          # Logs view
├── main.go
├── go.mod
├── Makefile
└── README.md
```

## Requirements

- Go 1.21+
- Access to a Kubernetes cluster
- Valid kubeconfig

## Tech Stack

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes client

## License

MIT License
