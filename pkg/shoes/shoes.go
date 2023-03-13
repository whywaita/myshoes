package shoes

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-plugin"

	pb "github.com/whywaita/myshoes/api/proto.go"
	"github.com/whywaita/myshoes/internal/config"
	"github.com/whywaita/myshoes/pkg/datastore"

	"google.golang.org/grpc"
)

// GetClient retrieve ShoesClient use shoes-plugin
func GetClient() (Client, func(), error) {
	Handshake := plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "SHOES_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "are_you_a_shoes?",
	}
	PluginMap := map[string]plugin.Plugin{
		"shoes_grpc": &Plugin{},
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          PluginMap,
		Cmd:              exec.Command(config.Config.ShoesPluginPath),
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

	return raw.(Client), client.Kill, nil
}

// Plugin is plugin implement
type Plugin struct {
	plugin.Plugin

	Impl Client
}

// GRPCServer is server
func (p *Plugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

// GRPCClient is client
func (p *Plugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: pb.NewShoesClient(c)}, nil
}

// Client is plugin client interface
type Client interface {
	AddInstance(ctx context.Context, runnerID, setupScript string, resourceType datastore.ResourceType, labels []string) (string, string, string, datastore.ResourceType, error)
	DeleteInstance(ctx context.Context, cloudID string, labels []string) error
}

// GRPCClient is plugin client implement
type GRPCClient struct {
	client pb.ShoesClient
}

// AddInstance create instance for runner
func (c *GRPCClient) AddInstance(ctx context.Context, runnerName, setupScript string, resourceType datastore.ResourceType, labels []string) (string, string, string, datastore.ResourceType, error) {
	req := &pb.AddInstanceRequest{
		RunnerName:   runnerName,
		SetupScript:  setupScript,
		ResourceType: resourceType.ToPb(),
		Labels:       labels,
	}
	resp, err := c.client.AddInstance(ctx, req)
	if err != nil {
		return "", "", "", datastore.ResourceType(0), fmt.Errorf("failed to AddInstance: %w", err)
	}

	return resp.CloudId, resp.IpAddress, resp.ShoesType, datastore.ResourceType(resp.ResourceType), nil
}

// DeleteInstance delete instance for runner
func (c *GRPCClient) DeleteInstance(ctx context.Context, cloudID string, labels []string) error {
	req := &pb.DeleteInstanceRequest{
		CloudId: cloudID,
		Labels:  labels,
	}
	_, err := c.client.DeleteInstance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to DeleteInstance: %w", err)
	}

	return nil
}
