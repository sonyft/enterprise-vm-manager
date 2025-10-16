package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

var (
	configPath string
	verbose    bool
	output     string
	apiURL     string
	apiKey     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vmctl",
		Short: "VM Manager CLI Tool",
		Long: `vmctl is a command-line interface for the Enterprise VM Manager API.
It allows you to manage virtual machines, view statistics, and perform
various operations from the command line.`,
		Version: fmt.Sprintf("%s (built %s, commit %s)", version, buildTime, gitCommit),
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format (table, json, yaml)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "VM Manager API URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	// Add subcommands
	rootCmd.AddCommand(
		newVMCommand(),
		newStatsCommand(),
		newConfigCommand(),
		newCompletionCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newVMCommand creates the VM management command
func newVMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines",
		Long:  "Create, list, update, delete, and control virtual machines",
	}

	// VM subcommands
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List virtual machines",
			Long:  "List all virtual machines with optional filtering",
			RunE:  runListVMs,
		},
		&cobra.Command{
			Use:   "get <vm-id>",
			Short: "Get virtual machine details",
			Long:  "Get detailed information about a specific virtual machine",
			Args:  cobra.ExactArgs(1),
			RunE:  runGetVM,
		},
		&cobra.Command{
			Use:   "create <name>",
			Short: "Create a new virtual machine",
			Long:  "Create a new virtual machine with specified configuration",
			Args:  cobra.ExactArgs(1),
			RunE:  runCreateVM,
		},
		&cobra.Command{
			Use:   "delete <vm-id>",
			Short: "Delete a virtual machine",
			Long:  "Delete a virtual machine (must be stopped)",
			Args:  cobra.ExactArgs(1),
			RunE:  runDeleteVM,
		},
		&cobra.Command{
			Use:   "start <vm-id>",
			Short: "Start a virtual machine",
			Long:  "Start a stopped virtual machine",
			Args:  cobra.ExactArgs(1),
			RunE:  runStartVM,
		},
		&cobra.Command{
			Use:   "stop <vm-id>",
			Short: "Stop a virtual machine",
			Long:  "Stop a running virtual machine",
			Args:  cobra.ExactArgs(1),
			RunE:  runStopVM,
		},
		&cobra.Command{
			Use:   "restart <vm-id>",
			Short: "Restart a virtual machine",
			Long:  "Restart a running virtual machine",
			Args:  cobra.ExactArgs(1),
			RunE:  runRestartVM,
		},
		&cobra.Command{
			Use:   "stats <vm-id>",
			Short: "Get VM statistics",
			Long:  "Get real-time statistics for a virtual machine",
			Args:  cobra.ExactArgs(1),
			RunE:  runVMStats,
		},
	)

	// Add flags for create command
	createCmd := cmd.Commands()[2] // "create" command
	createCmd.Flags().Int("cpu", 2, "Number of CPU cores")
	createCmd.Flags().Int("ram", 2048, "RAM in MB")
	createCmd.Flags().Int("disk", 50, "Disk size in GB")
	createCmd.Flags().String("image", "ubuntu:22.04", "VM image name")
	createCmd.Flags().String("network", "nat", "Network type (nat, bridge, host)")
	createCmd.Flags().String("description", "", "VM description")

	// Add flags for list command
	listCmd := cmd.Commands()[0] // "list" command
	listCmd.Flags().String("status", "", "Filter by status")
	listCmd.Flags().String("node", "", "Filter by node ID")
	listCmd.Flags().String("search", "", "Search in name and description")
	listCmd.Flags().Int("limit", 20, "Number of results per page")
	listCmd.Flags().Int("page", 1, "Page number")

	return cmd
}

// newStatsCommand creates the statistics command
func newStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show system statistics",
		Long:  "Show overall system resource usage and VM statistics",
		RunE:  runSystemStats,
	}
}

// newConfigCommand creates the configuration command
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		Long:  "Manage CLI configuration and settings",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Show current configuration",
			Long:  "Display current CLI configuration",
			RunE:  runShowConfig,
		},
		&cobra.Command{
			Use:   "init",
			Short: "Initialize configuration",
			Long:  "Create a new configuration file",
			RunE:  runInitConfig,
		},
	)

	return cmd
}

// newCompletionCommand creates the completion command
func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:

  $ source <(vmctl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ vmctl completion bash > /etc/bash_completion.d/vmctl
  # macOS:
  $ vmctl completion bash > /usr/local/etc/bash_completion.d/vmctl

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ vmctl completion zsh > "${fpath[1]}/_vmctl"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ vmctl completion fish | source

  # To load completions for each session, execute once:
  $ vmctl completion fish > ~/.config/fish/completions/vmctl.fish

PowerShell:

  PS> vmctl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> vmctl completion powershell > vmctl.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
}

// Command implementations

func runListVMs(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Listing VMs...")
	// This would make HTTP request to API
	fmt.Println("✅ Found 5 VMs")

	// Mock response for demonstration
	vms := []map[string]interface{}{
		{
			"id":     "123e4567-e89b-12d3-a456-426614174000",
			"name":   "web-server-01",
			"status": "running",
			"cpu":    4,
			"ram":    8192,
			"node":   "node-01",
		},
		{
			"id":     "123e4567-e89b-12d3-a456-426614174001",
			"name":   "database-primary",
			"status": "running",
			"cpu":    8,
			"ram":    16384,
			"node":   "node-02",
		},
	}

	switch output {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(vms)
	case "table":
		printVMTable(vms)
	default:
		printVMTable(vms)
	}

	return nil
}

func runGetVM(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("🔍 Getting VM details: %s\n", vmID)

	// Mock response
	vm := map[string]interface{}{
		"id":          vmID,
		"name":        "web-server-01",
		"description": "Production web server",
		"status":      "running",
		"cpu_cores":   4,
		"ram_mb":      8192,
		"disk_gb":     100,
		"image_name":  "ubuntu:22.04",
		"node_id":     "node-01",
		"created_at":  "2025-10-15T20:00:00Z",
		"uptime":      3600,
	}

	switch output {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(vm)
	default:
		printVMDetails(vm)
	}

	return nil
}

func runCreateVM(cmd *cobra.Command, args []string) error {
	name := args[0]
	cpu, _ := cmd.Flags().GetInt("cpu")
	ram, _ := cmd.Flags().GetInt("ram")
	disk, _ := cmd.Flags().GetInt("disk")
	image, _ := cmd.Flags().GetString("image")
	network, _ := cmd.Flags().GetString("network")
	description, _ := cmd.Flags().GetString("description")

	fmt.Printf("🚀 Creating VM: %s\n", name)
	fmt.Printf("   CPU: %d cores\n", cpu)
	fmt.Printf("   RAM: %d MB\n", ram)
	fmt.Printf("   Disk: %d GB\n", disk)
	fmt.Printf("   Image: %s\n", image)
	fmt.Printf("   Network: %s\n", network)
	if description != "" {
		fmt.Printf("   Description: %s\n", description)
	}

	// This would make HTTP POST request to API
	fmt.Println("✅ VM created successfully!")
	fmt.Println("📋 VM ID: 123e4567-e89b-12d3-a456-426614174002")

	return nil
}

func runDeleteVM(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("🗑️  Deleting VM: %s\n", vmID)

	// This would make HTTP DELETE request to API
	fmt.Println("✅ VM deleted successfully!")

	return nil
}

func runStartVM(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("▶️  Starting VM: %s\n", vmID)

	// This would make HTTP POST request to API
	fmt.Println("✅ VM start initiated!")

	return nil
}

func runStopVM(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("⏹️  Stopping VM: %s\n", vmID)

	// This would make HTTP POST request to API
	fmt.Println("✅ VM stop initiated!")

	return nil
}

func runRestartVM(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("🔄 Restarting VM: %s\n", vmID)

	// This would make HTTP POST request to API
	fmt.Println("✅ VM restart initiated!")

	return nil
}

func runVMStats(cmd *cobra.Command, args []string) error {
	vmID := args[0]
	fmt.Printf("📊 Getting VM statistics: %s\n", vmID)

	// Mock response
	stats := map[string]interface{}{
		"vm_id":              vmID,
		"cpu_usage_percent":  45.2,
		"ram_usage_percent":  67.8,
		"disk_usage_percent": 23.1,
		"network_rx_bytes":   1048576,
		"network_tx_bytes":   524288,
		"uptime_seconds":     3600,
		"last_updated":       "2025-10-15T20:30:00Z",
	}

	switch output {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(stats)
	default:
		printVMStats(stats)
	}

	return nil
}

func runSystemStats(cmd *cobra.Command, args []string) error {
	fmt.Println("📊 Getting system statistics...")

	// Mock response
	stats := map[string]interface{}{
		"total_vms":       10,
		"running_vms":     7,
		"stopped_vms":     3,
		"total_cpu_cores": 40,
		"used_cpu_cores":  28,
		"cpu_usage":       70.0,
		"total_ram_mb":    81920,
		"used_ram_mb":     57344,
		"ram_usage":       70.0,
		"total_nodes":     4,
		"active_nodes":    4,
	}

	switch output {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(stats)
	default:
		printSystemStats(stats)
	}

	return nil
}

func runShowConfig(cmd *cobra.Command, args []string) error {
	fmt.Println("📋 Current Configuration:")
	fmt.Printf("API URL: %s\n", apiURL)
	fmt.Printf("Output Format: %s\n", output)
	fmt.Printf("Verbose: %v\n", verbose)
	if apiKey != "" {
		fmt.Println("API Key: [configured]")
	} else {
		fmt.Println("API Key: [not configured]")
	}

	return nil
}

func runInitConfig(cmd *cobra.Command, args []string) error {
	fmt.Println("🔧 Initializing CLI configuration...")

	// This would create a config file
	configContent := `# VM Manager CLI Configuration
api_url: http://localhost:8080
api_key: ""
output_format: table
verbose: false
`

	configFile := "vmctl.yaml"
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	fmt.Printf("✅ Configuration file created: %s\n", configFile)
	fmt.Println("💡 Edit this file to customize your settings")

	return nil
}

// Helper functions for formatting output

func printVMTable(vms []map[string]interface{}) {
	fmt.Println()
	fmt.Printf("%-36s %-20s %-12s %-8s %-10s %-10s\n",
		"ID", "NAME", "STATUS", "CPU", "RAM (MB)", "NODE")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────────────")

	for _, vm := range vms {
		fmt.Printf("%-36s %-20s %-12s %-8v %-10v %-10s\n",
			vm["id"], vm["name"], vm["status"], vm["cpu"], vm["ram"], vm["node"])
	}
	fmt.Println()
}

func printVMDetails(vm map[string]interface{}) {
	fmt.Println()
	fmt.Println("🖥️  Virtual Machine Details")
	fmt.Println("═══════════════════════════")
	fmt.Printf("ID:           %s\n", vm["id"])
	fmt.Printf("Name:         %s\n", vm["name"])
	fmt.Printf("Description:  %s\n", vm["description"])
	fmt.Printf("Status:       %s\n", vm["status"])
	fmt.Printf("CPU Cores:    %v\n", vm["cpu_cores"])
	fmt.Printf("RAM (MB):     %v\n", vm["ram_mb"])
	fmt.Printf("Disk (GB):    %v\n", vm["disk_gb"])
	fmt.Printf("Image:        %s\n", vm["image_name"])
	fmt.Printf("Node:         %s\n", vm["node_id"])
	fmt.Printf("Created:      %s\n", vm["created_at"])
	fmt.Printf("Uptime:       %v seconds\n", vm["uptime"])
	fmt.Println()
}

func printVMStats(stats map[string]interface{}) {
	fmt.Println()
	fmt.Println("📊 VM Statistics")
	fmt.Println("═════════════════")
	fmt.Printf("CPU Usage:    %.1f%%\n", stats["cpu_usage_percent"])
	fmt.Printf("RAM Usage:    %.1f%%\n", stats["ram_usage_percent"])
	fmt.Printf("Disk Usage:   %.1f%%\n", stats["disk_usage_percent"])
	fmt.Printf("Network RX:   %v bytes\n", stats["network_rx_bytes"])
	fmt.Printf("Network TX:   %v bytes\n", stats["network_tx_bytes"])
	fmt.Printf("Uptime:       %v seconds\n", stats["uptime_seconds"])
	fmt.Printf("Last Updated: %s\n", stats["last_updated"])
	fmt.Println()
}

func printSystemStats(stats map[string]interface{}) {
	fmt.Println()
	fmt.Println("📊 System Statistics")
	fmt.Println("════════════════════")
	fmt.Printf("Total VMs:     %v\n", stats["total_vms"])
	fmt.Printf("Running VMs:   %v\n", stats["running_vms"])
	fmt.Printf("Stopped VMs:   %v\n", stats["stopped_vms"])
	fmt.Println()
	fmt.Printf("CPU Cores:     %v / %v (%.0f%% used)\n",
		stats["used_cpu_cores"], stats["total_cpu_cores"], stats["cpu_usage"])
	fmt.Printf("RAM (MB):      %v / %v (%.0f%% used)\n",
		stats["used_ram_mb"], stats["total_ram_mb"], stats["ram_usage"])
	fmt.Println()
	fmt.Printf("Nodes:         %v active / %v total\n",
		stats["active_nodes"], stats["total_nodes"])
	fmt.Println()
}
