package main

import (
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/client"
	"github.com/spf13/cobra"
)

var (
	flagServer string
	flagAPIKey string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cpa-client",
	Short: "CLI client for CLIProxyAPI Plus",
	Long:  "cpa-client - CLI client for CLIProxyAPI Plus management API",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagServer, "server", "", "Server URL (e.g. http://127.0.0.1:8317)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "Management API key")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(kiroUsageCmd)
	rootCmd.AddCommand(uploadKiroCmd)
}

// resolveConfig loads saved config and applies flag overrides.
// Returns an error if server or apiKey are missing.
func resolveConfig() (client.Config, error) {
	cfg, err := client.LoadConfig()
	if err != nil {
		return client.Config{}, fmt.Errorf("loading config: %w", err)
	}
	if flagServer != "" {
		cfg.Server = flagServer
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if cfg.Server == "" {
		return client.Config{}, fmt.Errorf("no server configured. Run: cpa-client config --server URL --api-key KEY")
	}
	if cfg.APIKey == "" {
		return client.Config{}, fmt.Errorf("no API key configured. Run: cpa-client config --api-key KEY")
	}
	return cfg, nil
}

// --- config command ---

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Save or display server connection settings",
	Long: `Save or display server connection settings.

When called without flags, displays the current saved config.
When called with --server and/or --api-key, saves them for future use.`,
	Example: `  cpa-client config --server http://127.0.0.1:8317 --api-key YOUR_KEY
  cpa-client config`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagServer == "" && flagAPIKey == "" {
			cfg, err := client.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if cfg.Server == "" && cfg.APIKey == "" {
				fmt.Println("No config saved yet. Use --server and --api-key to set.")
				return nil
			}
			fmt.Printf("Server:  %s\n", cfg.Server)
			fmt.Printf("API Key: %s\n", cfg.APIKey)
			return nil
		}

		cfg, err := client.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if flagServer != "" {
			cfg.Server = flagServer
		}
		if flagAPIKey != "" {
			cfg.APIKey = flagAPIKey
		}

		if err := client.SaveConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		path, _ := client.DefaultConfigPath()
		fmt.Printf("Config saved to %s\n", path)
		return nil
	},
}

// --- kiro-usage command ---

var kiroUsageJSONFlag bool

var kiroUsageCmd = &cobra.Command{
	Use:   "kiro-usage",
	Short: "Show Kiro account credit usage",
	Example: `  cpa-client kiro-usage
  cpa-client kiro-usage --json
  cpa-client kiro-usage --server http://127.0.0.1:8317 --api-key KEY`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}

		accounts, err := client.FetchKiroUsage(cfg.Server, cfg.APIKey)
		if err != nil {
			return err
		}

		if kiroUsageJSONFlag {
			return client.PrintKiroUsageJSON(accounts)
		}
		client.PrintKiroUsageTable(accounts)
		return nil
	},
}

func init() {
	kiroUsageCmd.Flags().BoolVar(&kiroUsageJSONFlag, "json", false, "Output as JSON")
}

// --- upload-kiro command ---

var uploadKiroFileFlag string

var uploadKiroCmd = &cobra.Command{
	Use:   "upload-kiro",
	Short: "Upload Kiro credentials to the management API",
	Long: `Upload Kiro credentials in camelCase JSON format (from Kiro Account Manager)
to the CLIProxyAPI management server.

Accepts a JSON array or single object. Reads from --file or stdin.`,
	Example: `  cpa-client upload-kiro --file credentials.json
  cpa-client upload-kiro --file credentials.json --server http://127.0.0.1:8317 --api-key KEY
  cat credentials.json | cpa-client upload-kiro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}

		var data []byte
		if uploadKiroFileFlag != "" {
			data, err = os.ReadFile(uploadKiroFileFlag)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
		} else {
			data, err = client.ReadKiroInputFromStdin()
			if err != nil {
				return err
			}
		}

		creds, err := client.ParseKiroInput(data)
		if err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}

		if len(creds) == 0 {
			return fmt.Errorf("no credentials found in input")
		}

		fmt.Fprintf(os.Stderr, "Found %d credential(s) to upload\n", len(creds))

		for i, ext := range creds {
			internal := client.ConvertKiroCredential(ext)
			fileName := client.GenerateKiroFileName(ext, i)

			fmt.Fprintf(os.Stderr, "  [%d/%d] Uploading %s ...", i+1, len(creds), fileName)
			if err := client.UploadKiroCredential(cfg.Server, cfg.APIKey, fileName, internal); err != nil {
				fmt.Fprintln(os.Stderr, " FAILED")
				return fmt.Errorf("uploading %s: %w", fileName, err)
			}
			fmt.Fprintln(os.Stderr, " OK")
		}

		fmt.Fprintf(os.Stderr, "Done. Uploaded %d credential(s).\n", len(creds))
		return nil
	},
}

func init() {
	uploadKiroCmd.Flags().StringVarP(&uploadKiroFileFlag, "file", "f", "", "Path to JSON file with Kiro credentials")
}
