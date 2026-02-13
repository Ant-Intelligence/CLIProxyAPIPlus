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

var (
	uploadKiroFileFlag      string
	uploadKiroTokenFileFlag string
	uploadKiroClientFile    string
	uploadKiroNameFlag      string
)

var uploadKiroCmd = &cobra.Command{
	Use:   "upload-kiro",
	Short: "Upload Kiro credentials to the management API",
	Long: `Upload Kiro credentials to the CLIProxyAPI management server.

Two modes:

  1. Kiro Account Manager format (--file):
     Accepts a JSON array or single object in camelCase format.

  2. AWS SSO cache files (--token-file):
     Reads the token file and its companion client file (auto-discovered
     from clientIdHash, or specified with --client-file).
     Supports both relative and absolute paths, including ~/...

If neither --file nor --token-file is given, reads from stdin.`,
	Example: `  # Kiro Account Manager format
  cpa-client upload-kiro --file credentials.json

  # AWS SSO cache files (auto-discover client file from clientIdHash)
  cpa-client upload-kiro --token-file ~/.aws/sso/cache/kiro-auth-token.json --name kiro-idc.json

  # AWS SSO cache files (explicit client file)
  cpa-client upload-kiro --token-file ./token.json --client-file ./client.json --name kiro-idc.json

  # Pipe from stdin
  cat credentials.json | cpa-client upload-kiro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}

		// Mode 2: AWS SSO cache files
		if uploadKiroTokenFileFlag != "" {
			return runUploadKiroSSO(cfg)
		}

		// Mode 1: Kiro Account Manager format (--file or stdin)
		return runUploadKiroAccountManager(cfg)
	},
}

func runUploadKiroSSO(cfg client.Config) error {
	fmt.Fprintln(os.Stderr, "Reading AWS SSO cache files...")

	cred, err := client.ReadKiroSSOFiles(uploadKiroTokenFileFlag, uploadKiroClientFile)
	if err != nil {
		return err
	}

	fileName := uploadKiroNameFlag
	if fileName == "" {
		fileName = "kiro-idc.json"
	}

	fmt.Fprintf(os.Stderr, "  Provider:    %s\n", cred.Provider)
	fmt.Fprintf(os.Stderr, "  AuthMethod:  %s\n", cred.AuthMethod)
	fmt.Fprintf(os.Stderr, "  Region:      %s\n", cred.Region)
	if cred.ClientID != "" {
		fmt.Fprintln(os.Stderr, "  ClientID:    (present)")
	}
	if cred.ClientSecret != "" {
		fmt.Fprintln(os.Stderr, "  ClientSecret:(present)")
	}

	fmt.Fprintf(os.Stderr, "Uploading as %s ...", fileName)
	if err := client.UploadKiroCredential(cfg.Server, cfg.APIKey, fileName, cred); err != nil {
		fmt.Fprintln(os.Stderr, " FAILED")
		return fmt.Errorf("uploading %s: %w", fileName, err)
	}
	fmt.Fprintln(os.Stderr, " OK")
	fmt.Fprintln(os.Stderr, "Done.")
	return nil
}

func runUploadKiroAccountManager(cfg client.Config) error {
	var (
		data []byte
		err  error
	)
	if uploadKiroFileFlag != "" {
		data, err = os.ReadFile(client.ResolvePath(uploadKiroFileFlag))
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
		fileName := uploadKiroNameFlag
		if fileName == "" {
			fileName = client.GenerateKiroFileName(ext, i)
		}

		fmt.Fprintf(os.Stderr, "  [%d/%d] Uploading %s ...", i+1, len(creds), fileName)
		if err := client.UploadKiroCredential(cfg.Server, cfg.APIKey, fileName, internal); err != nil {
			fmt.Fprintln(os.Stderr, " FAILED")
			return fmt.Errorf("uploading %s: %w", fileName, err)
		}
		fmt.Fprintln(os.Stderr, " OK")
	}

	fmt.Fprintf(os.Stderr, "Done. Uploaded %d credential(s).\n", len(creds))
	return nil
}

func init() {
	uploadKiroCmd.Flags().StringVarP(&uploadKiroFileFlag, "file", "f", "", "Path to Kiro Account Manager JSON file")
	uploadKiroCmd.Flags().StringVarP(&uploadKiroTokenFileFlag, "token-file", "t", "", "Path to AWS SSO token file (e.g. ~/.aws/sso/cache/kiro-auth-token.json)")
	uploadKiroCmd.Flags().StringVarP(&uploadKiroClientFile, "client-file", "c", "", "Path to AWS SSO client file (auto-discovered from clientIdHash if omitted)")
	uploadKiroCmd.Flags().StringVarP(&uploadKiroNameFlag, "name", "n", "", "Upload filename (default: auto-generated or kiro-idc.json)")
	uploadKiroCmd.MarkFlagsMutuallyExclusive("file", "token-file")
}
