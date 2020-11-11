package pluginutils

import (
	"context"

	pb "github.com/whywaita/myshoes/api/proto"
)

// Health is implement of health check via gRPC
type Health struct{}

// Check is health check
func (h *Health) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// all ok
	return &pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_SERVING,
	}, nil
}
