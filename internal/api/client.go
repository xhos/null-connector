package api

import (
	"context"
	"fmt"
	"math"

	"null-connector/internal/domain"
	pb "null-connector/internal/gen/null/v1"

	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	conn         *grpc.ClientConn
	txClient     pb.TransactionServiceClient
	healthClient grpc_health_v1.HealthClient

	authToken string
	userID    string
}

func NewClient(nullCoreURL, authToken, userID string) (*Client, error) {
	conn, err := grpc.NewClient(nullCoreURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial null-core: %w", err)
	}

	return &Client{
		conn:         conn,
		txClient:     pb.NewTransactionServiceClient(conn),
		healthClient: grpc_health_v1.NewHealthClient(conn),
		authToken:    authToken,
		userID:       userID,
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
		return fmt.Errorf("ping null-core: %w", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("null-core not serving: %s", resp.Status)
	}
	return nil
}

// CreateTransactions posts a batch to null-core
func (c *Client) CreateTransactions(ctx context.Context, txs []domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	inputs := make([]*pb.TransactionInput, 0, len(txs))
	for _, tx := range txs {
		inputs = append(inputs, toInput(tx))
	}

	_, err := c.txClient.CreateTransaction(c.withAuth(ctx), &pb.CreateTransactionRequest{
		UserId:       c.userID,
		Transactions: inputs,
	})
	if err != nil {
		// AlreadyExists from the server is treated as success
		// duplicate detection is null-core's job
		if status.Code(err) == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("create transactions: %w", err)
	}
	return nil
}

func (c *Client) withAuth(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-internal-key", c.authToken))
}

func toInput(tx domain.Transaction) *pb.TransactionInput {
	in := &pb.TransactionInput{
		AccountId: tx.AccountID,
		TxDate:    timestamppb.New(tx.Date),
		TxAmount:  amountToMoney(tx.Amount, tx.Currency),
		Direction: directionToProto(tx.Direction),
	}
	if tx.ExternalID != "" {
		in.ExternalId = &tx.ExternalID
	}
	if tx.Description != "" {
		in.Description = &tx.Description
	}
	if tx.Merchant != "" {
		in.Merchant = &tx.Merchant
	}
	return in
}

func amountToMoney(amount float64, currency string) *money.Money {
	units := int64(amount)
	nanos := int32(math.Round((amount - float64(units)) * 1e9))
	return &money.Money{
		CurrencyCode: currency,
		Units:        units,
		Nanos:        nanos,
	}
}

func directionToProto(d domain.Direction) pb.TransactionDirection {
	switch d {
	case domain.DirectionIn:
		return pb.TransactionDirection_DIRECTION_INCOMING
	case domain.DirectionOut:
		return pb.TransactionDirection_DIRECTION_OUTGOING
	default:
		return pb.TransactionDirection_DIRECTION_UNSPECIFIED
	}
}
