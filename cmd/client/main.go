package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		runConfig(os.Args[2:])
	case "kiro-usage":
		runKiroUsage(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `cpa-client - CLI client for CLIProxyAPI Plus

Usage:
  cpa-client <command> [flags]

Commands:
  config       Save server connection settings
  kiro-usage   Show Kiro account credit usage

Examples:
  cpa-client config --server http://127.0.0.1:8317 --api-key YOUR_KEY
  cpa-client kiro-usage
  cpa-client kiro-usage --json`)
}

func runConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	server := fs.String("server", "", "Server URL (e.g. http://127.0.0.1:8317)")
	apiKey := fs.String("api-key", "", "Management API key")
	fs.Parse(args)

	if *server == "" && *apiKey == "" {
		// Show current config
		cfg, err := client.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		if cfg.Server == "" && cfg.APIKey == "" {
			fmt.Println("No config saved yet. Use --server and --api-key to set.")
			return
		}
		fmt.Printf("Server:  %s\n", cfg.Server)
		fmt.Printf("API Key: %s\n", cfg.APIKey)
		return
	}

	cfg, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if *server != "" {
		cfg.Server = *server
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}

	if err := client.SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	path, _ := client.DefaultConfigPath()
	fmt.Printf("Config saved to %s\n", path)
}

func runKiroUsage(args []string) {
	fs := flag.NewFlagSet("kiro-usage", flag.ExitOnError)
	server := fs.String("server", "", "Server URL (overrides saved config)")
	apiKey := fs.String("api-key", "", "Management API key (overrides saved config)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	cfg, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Flag overrides take precedence
	if *server != "" {
		cfg.Server = *server
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}

	if cfg.Server == "" {
		fmt.Fprintln(os.Stderr, "Error: no server configured. Run: cpa-client config --server URL --api-key KEY")
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: no API key configured. Run: cpa-client config --api-key KEY")
		os.Exit(1)
	}

	accounts, err := client.FetchKiroUsage(cfg.Server, cfg.APIKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		if err := client.PrintKiroUsageJSON(accounts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	client.PrintKiroUsageTable(accounts)
}
