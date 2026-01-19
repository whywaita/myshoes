package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/go-plugin"
	"github.com/whywaita/myshoes/pkg/config"
	"github.com/whywaita/myshoes/pkg/datastore"
	"github.com/whywaita/myshoes/pkg/gh"
	"github.com/whywaita/myshoes/pkg/shoes"
	"github.com/whywaita/myshoes/pkg/starter"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "add":
		if err := runAdd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "delete":
		if err := runDelete(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: shoes-tester <command> [options]

Commands:
  add       Add an instance
  delete    Delete an instance

Run 'shoes-tester <command> --help' for more information on a command.
`)
}

type addFlags struct {
	pluginPath           string
	runnerName           string
	resourceType         string
	labels               string
	setupScript          string
	generateScript       bool
	scope                string
	githubAppID          string
	githubPrivateKeyPath string
	runnerVersion        string
	runnerUser           string
	runnerBaseDirectory  string
	githubURL            string
	jsonOutput           bool
}

type deleteFlags struct {
	pluginPath string
	cloudID    string
	labels     string
	jsonOutput bool
}

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	flags := &addFlags{}

	fs.StringVar(&flags.pluginPath, "plugin", "", "Path to shoes-provider binary (required)")
	fs.StringVar(&flags.runnerName, "runner-name", "", "Runner name (required)")
	fs.StringVar(&flags.resourceType, "resource-type", "nano", "Resource type (nano|micro|small|medium|large|xlarge|2xlarge|3xlarge|4xlarge)")
	fs.StringVar(&flags.labels, "labels", "", "Comma-separated labels")
	fs.StringVar(&flags.setupScript, "setup-script", "", "Setup script (simple mode)")
	fs.BoolVar(&flags.generateScript, "generate-script", false, "Generate setup script (script generation mode)")
	fs.StringVar(&flags.scope, "scope", "", "Repository (owner/repo) or Organization (script generation mode)")
	fs.StringVar(&flags.githubAppID, "github-app-id", os.Getenv("GITHUB_APP_ID"), "GitHub App ID (script generation mode)")
	fs.StringVar(&flags.githubPrivateKeyPath, "github-private-key-path", os.Getenv("GITHUB_PRIVATE_KEY_PATH"), "GitHub App private key path (script generation mode)")
	fs.StringVar(&flags.runnerVersion, "runner-version", "latest", "Runner version (script generation mode)")
	fs.StringVar(&flags.runnerUser, "runner-user", "runner", "Runner user (script generation mode)")
	fs.StringVar(&flags.runnerBaseDirectory, "runner-base-directory", "/tmp", "Runner base directory (script generation mode)")
	fs.StringVar(&flags.githubURL, "github-url", "", "GitHub Enterprise Server URL (script generation mode)")
	fs.BoolVar(&flags.jsonOutput, "json", false, "Output in JSON format")

	fs.Parse(args)

	if flags.pluginPath == "" {
		return fmt.Errorf("--plugin is required")
	}
	if flags.runnerName == "" {
		return fmt.Errorf("--runner-name is required")
	}

	if flags.generateScript {
		if flags.scope == "" {
			return fmt.Errorf("--scope is required for script generation mode")
		}
		if flags.githubAppID == "" {
			return fmt.Errorf("--github-app-id is required for script generation mode")
		}
		if flags.githubPrivateKeyPath == "" {
			return fmt.Errorf("--github-private-key-path is required for script generation mode")
		}
	}

	ctx := context.Background()

	var setupScript string
	if flags.generateScript {
		script, err := generateSetupScript(ctx, flags)
		if err != nil {
			return fmt.Errorf("failed to generate setup script: %w", err)
		}
		setupScript = script
	} else {
		setupScript = flags.setupScript
	}

	resourceType := datastore.UnmarshalResourceTypeString(flags.resourceType)
	if resourceType == datastore.ResourceTypeUnknown && flags.resourceType != "unknown" {
		return fmt.Errorf("invalid resource type: %s", flags.resourceType)
	}

	labels := parseLabels(flags.labels)

	client, teardown, err := getClientWithPath(flags.pluginPath)
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	cloudID, ipAddress, shoesType, actualResourceType, err := client.AddInstance(ctx, flags.runnerName, setupScript, resourceType, labels)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	if flags.jsonOutput {
		output := map[string]string{
			"cloud_id":      cloudID,
			"ip_address":    ipAddress,
			"shoes_type":    shoesType,
			"resource_type": actualResourceType.String(),
		}
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("AddInstance succeeded:\n")
		fmt.Printf("  Cloud ID:      %s\n", cloudID)
		fmt.Printf("  Shoes Type:    %s\n", shoesType)
		fmt.Printf("  IP Address:    %s\n", ipAddress)
		fmt.Printf("  Resource Type: %s\n", actualResourceType.String())
	}

	return nil
}

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	flags := &deleteFlags{}

	fs.StringVar(&flags.pluginPath, "plugin", "", "Path to shoes-provider binary (required)")
	fs.StringVar(&flags.cloudID, "cloud-id", "", "Cloud ID (required)")
	fs.StringVar(&flags.labels, "labels", "", "Comma-separated labels")
	fs.BoolVar(&flags.jsonOutput, "json", false, "Output in JSON format")

	fs.Parse(args)

	if flags.pluginPath == "" {
		return fmt.Errorf("--plugin is required")
	}
	if flags.cloudID == "" {
		return fmt.Errorf("--cloud-id is required")
	}

	ctx := context.Background()

	labels := parseLabels(flags.labels)

	client, teardown, err := getClientWithPath(flags.pluginPath)
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}
	defer teardown()

	if err := client.DeleteInstance(ctx, flags.cloudID, labels); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	if flags.jsonOutput {
		output := map[string]string{
			"status": "success",
		}
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("DeleteInstance succeeded\n")
	}

	return nil
}

func getClientWithPath(pluginPath string) (shoes.Client, func(), error) {
	handshake := plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "SHOES_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "are_you_a_shoes?",
	}
	pluginMap := map[string]plugin.Plugin{
		"shoes_grpc": &shoes.Plugin{},
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          pluginMap,
		Cmd:              exec.Command(pluginPath),
		Managed:          true,
		Stderr:           os.Stderr,
		SyncStdout:       os.Stdout,
		SyncStderr:       os.Stderr,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get shoes client: %w", err)
	}

	raw, err := rpcClient.Dispense("shoes_grpc")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to shoes client instance: %w", err)
	}

	return raw.(shoes.Client), client.Kill, nil
}

func generateSetupScript(ctx context.Context, flags *addFlags) (string, error) {
	keyBytes, err := os.ReadFile(flags.githubPrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read private key: %w", err)
	}

	appID, err := strconv.ParseInt(flags.githubAppID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse github-app-id: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	config.Config.GitHub.AppID = appID
	config.Config.GitHub.PEMByte = keyBytes
	config.Config.GitHub.PEM = privateKey

	if flags.githubURL == "" {
		config.Config.GitHubURL = "https://github.com"
	} else {
		config.Config.GitHubURL = flags.githubURL
	}

	config.Config.RunnerUser = flags.runnerUser
	config.Config.RunnerBaseDirectory = flags.runnerBaseDirectory

	if err := gh.InitializeCache(appID, keyBytes); err != nil {
		return "", fmt.Errorf("failed to initialize GitHub client cache: %w", err)
	}

	s := starter.New(nil, nil, flags.runnerVersion, nil)
	return s.GetSetupScript(ctx, flags.scope, flags.runnerName)
}

func parseLabels(labels string) []string {
	if labels == "" {
		return []string{}
	}
	parts := strings.Split(labels, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
