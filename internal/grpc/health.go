package grpc

import (
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type HealthServer struct {
	server    *grpc.Server
	healthSrv *health.Server
	listener  net.Listener
}

func NewHealthServer(addr string) (*HealthServer, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	server := grpc.NewServer()
	healthSrv := health.NewServer()

	grpc_health_v1.RegisterHealthServer(server, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	return &HealthServer{
		server:    server,
		healthSrv: healthSrv,
		listener:  lis,
	}, nil
}

func (h *HealthServer) Start() error {
	return h.server.Serve(h.listener)
}

func (h *HealthServer) Stop() {
	h.server.GracefulStop()
}

func (h *HealthServer) SetStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	h.healthSrv.SetServingStatus(service, status)
}
