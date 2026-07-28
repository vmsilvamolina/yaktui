package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/vmsilvamolina/yaktui/internal/addons"
	"github.com/vmsilvamolina/yaktui/internal/client"
	"github.com/vmsilvamolina/yaktui/internal/config"
	"github.com/vmsilvamolina/yaktui/internal/tui"
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

	// Load and activate addons (e.g. Kyverno policies). Neither step is
	// fatal: a config typo or a cluster without discovery access must not
	// prevent yaktui from starting.
	addonDefs, err := config.LoadAddonDefinitions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load addon config: %v\n", err)
		addonDefs = addons.Builtins
	}
	detectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	detectedGroups, err := k8sClient.DetectAPIGroups(detectCtx)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to detect cluster API groups: %v\n", err)
		detectedGroups = map[string]bool{}
	}
	activeAddons := addons.ActivateDefinitions(addonDefs, detectedGroups)

	// Create and run the Bubbletea app
	// Connection check happens inside the app
	model := tui.NewModel(k8sClient, kubeconfig, activeAddons)
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
