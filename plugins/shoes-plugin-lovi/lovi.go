package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/whywaita/myshoes/pkg/pluginutils"

	pb "github.com/whywaita/myshoes/api/proto"
)

const (
	listenAddress = "127.0.0.1:8080"
)

func main() {
	grpcServer, lis, handshakeBody, err := pluginutils.Setup(listenAddress)
	if err != nil {
		log.Fatal(err)
	}

	p := &LoviPlugin{}
	pb.RegisterShoesServer(grpcServer, p)

	go func() {
		err = grpcServer.Serve(*lis)
		if err != nil {
			log.Fatalf("failed to serve gRPC Server: %+v\n", err)
		}
	}()
	defer grpcServer.Stop()

	fmt.Printf(handshakeBody)

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
}

type LoviPlugin struct{}

func (p *LoviPlugin) AddInstance(ctx context.Context, req *pb.AddInstanceRequest) (*pb.AddInstanceResponse, error) {
	return &pb.AddInstanceResponse{}, nil
}

func (p *LoviPlugin) DeleteInstance(ctx context.Context, req *pb.DeleteInstanceRequest) (*pb.DeleteInstanceResponse, error) {
	return &pb.DeleteInstanceResponse{}, nil
}
