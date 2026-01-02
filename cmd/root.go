package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/victor/yaktui/internal/client"
	"github.com/victor/yaktui/internal/tui"
)

var (
	kubeconfig string
	namespace  string
	version    = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "yaktui",
	Short: "YAKTUI - Yet Another KUBErnetes TUI",
	Long: `YAKTUI is a minimal terminal UI for Kubernetes.
Navigate your cluster with ease using a simple and intuitive interface.`,
	RunE: run,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("YAKTUI version %s\n", version)
	},
}

func init() {
	// Default kubeconfig path
	home, _ := os.UserHomeDir()
	defaultKubeconfig := filepath.Join(home, ".kube", "config")

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "Path to kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (default: default)")

	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Error is already printed by Cobra, just exit
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Check kubeconfig env var
	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" && kubeconfig == "" {
		kubeconfig = envKubeconfig
	}

	// Create Kubernetes client
	k8sClient, err := client.New(kubeconfig, namespace)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create and run the Bubbletea app
	// Connection check happens inside the app
	model := tui.NewModel(k8sClient, kubeconfig)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running application: %w", err)
	}

	return nil
}

// SetVersion sets the version for display
func SetVersion(v string) {
	version = v
}
