package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"

	"github.com/gophercloud/gophercloud"
	"github.com/whywaita/myshoes/pkg/pluginutils"

	"github.com/gophercloud/gophercloud/openstack"
	pb "github.com/whywaita/myshoes/api/proto"
)

const (
	DefaultPort   = "8080"
	DefaultRegion = "RegionOne"

	EnvPort      = "PORT"
	EnvFlavorID  = "OS_FLAVOR_ID"
	EnvImageID   = "OS_IMAGE_ID"
	EnvNetworkID = "OS_NETWORK_ID"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	c, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	grpcServer, lis, handshakeBody, err := pluginutils.Setup(c.listenAddress)
	if err != nil {
		return fmt.Errorf("failed to setup plugin: %w", err)
	}

	p, err := New(c)
	if err != nil {
		return fmt.Errorf("failed to setup openstack client: %w", err)
	}
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

	return nil
}

type OpenStackPlugin struct {
	computeClient *gophercloud.ServiceClient

	flavorID  string
	imageID   string
	networkID string
}

func New(c config) (*OpenStackPlugin, error) {
	p := &OpenStackPlugin{
		flavorID:  c.flavorID,
		imageID:   c.imageID,
		networkID: c.networkID,
	}

	computeClient, err := openstackAuthenticate()
	if err != nil {
		return nil, fmt.Errorf("failed to auth openstack: %w", err)
	}
	p.computeClient = computeClient

	return p, nil
}

func (p *OpenStackPlugin) AddInstance(ctx context.Context, req *pb.AddInstanceRequest) (*pb.AddInstanceResponse, error) {
	createOpts := servers.CreateOpts{
		Name:      req.RunnerId,
		FlavorRef: p.flavorID,
		ImageRef:  p.imageID,
		Networks:  []servers.Network{{UUID: p.networkID}},
		UserData:  []byte(req.SetupScript),
	}

	server, err := servers.Create(p.computeClient, createOpts).Extract()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create server: %+v", err)
	}

	return &pb.AddInstanceResponse{
		CloudId: server.ID,
	}, nil
}

func (p *OpenStackPlugin) DeleteInstance(ctx context.Context, req *pb.DeleteInstanceRequest) (*pb.DeleteInstanceResponse, error) {
	err := servers.Delete(p.computeClient, req.CloudId).ExtractErr()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete server: %+v", err)
	}

	return &pb.DeleteInstanceResponse{}, nil
}

type config struct {
	listenAddress string

	flavorID  string
	imageID   string
	networkID string
}

func loadConfig() (config, error) {
	var c config

	var port string
	if os.Getenv(EnvPort) == "" {
		port = DefaultPort
	} else {
		port = os.Getenv(EnvPort)
	}
	c.listenAddress = fmt.Sprintf("127.0.0.1:%s", port)

	if os.Getenv(EnvFlavorID) == "" || os.Getenv(EnvImageID) == "" || os.Getenv(EnvNetworkID) == "" {
		return config{}, fmt.Errorf("must be set instance ids")
	}
	c.flavorID = os.Getenv(EnvFlavorID)
	c.imageID = os.Getenv(EnvImageID)
	c.networkID = os.Getenv(EnvNetworkID)

	return c, nil
}

// openstackAuthenticate is auth
func openstackAuthenticate() (*gophercloud.ServiceClient, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	opts.DomainName = os.Getenv("OS_USER_DOMAIN_NAME")

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}

	computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, err
	}

	return computeClient, nil
}
