package api

import (
	"context"
	"fmt"

	pb "null-connector/internal/gen/null/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	conn          *grpc.ClientConn
	accountClient pb.AccountServiceClient
	txClient      pb.TransactionServiceClient
	userClient    pb.UserServiceClient
	healthClient  grpc_health_v1.HealthClient
	authToken     string
}

func NewClient(nullCoreURL, authToken string) (*Client, error) {
	conn, err := grpc.NewClient(nullCoreURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &Client{
		conn:          conn,
		accountClient: pb.NewAccountServiceClient(conn),
		txClient:      pb.NewTransactionServiceClient(conn),
		userClient:    pb.NewUserServiceClient(conn),
		healthClient:  grpc_health_v1.NewHealthClient(conn),
		authToken:     authToken,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "null.v1.UserService",
	})
	if err != nil {
		return fmt.Errorf("failed to ping null-core: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("null-core service not healthy: %s", resp.Status)
	}

	return nil
}

func (c *Client) withAuth(ctx context.Context) context.Context {
	md := metadata.Pairs("x-internal-key", c.authToken)
	return metadata.NewOutgoingContext(ctx, md)
}
