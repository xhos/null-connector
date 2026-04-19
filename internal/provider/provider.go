// Package provider defines the contract a transaction source must satisfy.
//
// TODO: change this comment once webhooks are supported.
// Today only Poll is exercised. Webhook delivery (Wise + SnapTrade both
// support it) will be added as a separate optional capability — likely a
// WebhookHandler interface that providers implement when their source can
// push events. The runner will accept either; both paths produce the same
// []domain.Transaction so downstream code stays uniform.
package provider

import (
	"context"

	"null-connector/internal/domain"
)

type Provider interface {
	// Name identifies the provider in logs.
	Name() string

	// Poll fetches transactions that have not yet been delivered to null-core.
	// An empty slice with nil error means "no new transactions"; this is the
	// expected steady state and must not be treated as an error.
	Poll(ctx context.Context) ([]domain.Transaction, error)
}
