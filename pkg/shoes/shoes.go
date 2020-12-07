package shoes

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-plugin"
	pb "github.com/whywaita/myshoes/api/proto"
	"github.com/whywaita/myshoes/internal/config"
	"google.golang.org/grpc"
)

// GetClient retrieve ShoesClient use shoes-plugin
func GetClient() (ShoesClient, func(), error) {
	Handshake := plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "SHOES_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "are_you_a_shoes?",
	}
	PluginMap := map[string]plugin.Plugin{
		"shoes_grpc": &ShoesPlugin{},
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

	return raw.(ShoesClient), client.Kill, nil
}

// ShoesPlugin is plugin implement
type ShoesPlugin struct {
	plugin.Plugin

	Impl ShoesClient
}

// GRPCServer is server
func (p *ShoesPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}

// GRPCClient is client
func (p *ShoesPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: pb.NewShoesClient(c)}, nil
}

// ShoesClient is plugin client interface
type ShoesClient interface {
	AddInstance(ctx context.Context, runnerID, setupScript string) error
	DeleteInstance(ctx context.Context, cloudID string) error
}

// GRPCClient is plugin client implement
type GRPCClient struct {
	client pb.ShoesClient
}

// AddInstance create instance for runner
func (c *GRPCClient) AddInstance(ctx context.Context, runnerID, setupScript string) error {
	req := &pb.AddInstanceRequest{
		RunnerId:    runnerID,
		SetupScript: setupScript,
	}
	resp, err := c.client.AddInstance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to AddInstance: %w", err)
	}

	fmt.Printf("%+v\n", resp)

	return nil
}

// DeleteInstance delete instance for runner
func (c *GRPCClient) DeleteInstance(ctx context.Context, cloudID string) error {
	req := &pb.DeleteInstanceRequest{
		CloudId: cloudID,
	}
	_, err := c.client.DeleteInstance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to DeleteInstance: %w", err)
	}

	return nil
}
