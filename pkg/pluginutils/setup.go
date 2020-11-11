package pluginutils

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/whywaita/myshoes/api/proto/plugin"
	"google.golang.org/grpc"
)

func Setup(listenAddress string) (*grpc.Server, *net.Listener, string, error) {
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to listen port: %w", err)
	}

	grpcServer := grpc.NewServer()

	stdioServer, err := NewGRPCStdioServer()
	if err != nil {
		log.Fatal(err)
	}
	plugin.RegisterGRPCStdioServer(grpcServer, stdioServer)
	healthpb.RegisterHealthServer(grpcServer, health.NewServer())

	handshakeBody := fmt.Sprintf("1|1|tcp|%s|grpc\n", listenAddress)

	return grpcServer, &lis, handshakeBody, nil
}
