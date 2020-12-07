package main

import (
	"context"
	"os/exec"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-plugin"
	pb "github.com/whywaita/myshoes/api/proto"
)

func main() {
	handshake := plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "SHOES_PLUGIN_MAGIC_COOKIE",
		MagicCookieValue: "are_you_a_shoes?",
	}
	pluginMap := map[string]plugin.Plugin{
		"shoes_grpc": &MockPlugin{},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}

type MockPlugin struct {
	plugin.Plugin
}

func (p *MockPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	server := Mock{}
	pb.RegisterShoesServer(s, server)

	return nil
}

func (p *MockPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return nil, nil
}

type Mock struct{}

func (m Mock) AddInstance(ctx context.Context, req *pb.AddInstanceRequest) (*pb.AddInstanceResponse, error) {
	if err := exec.Command("touch", "./AddInstance").Run(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to exec AddInstance: %+v", err)
	}
	return &pb.AddInstanceResponse{
		CloudId: "uuid",
	}, nil
}

func (m Mock) DeleteInstance(ctx context.Context, req *pb.DeleteInstanceRequest) (*pb.DeleteInstanceResponse, error) {
	if err := exec.Command("touch", "DeleteInstance").Run(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to exec DeleteInstance: %+v", err)
	}

	return &pb.DeleteInstanceResponse{}, nil
}
