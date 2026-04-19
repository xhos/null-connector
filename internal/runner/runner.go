package runner

import (
	"context"

	"null-connector/internal/domain"
	"null-connector/internal/provider"

	"github.com/charmbracelet/log"
)

// Sink receives transactions for delivery to null-core
type Sink interface {
	CreateTransactions(ctx context.Context, txs []domain.Transaction) error
}

type Runner struct {
	providers []provider.Provider
	sink      Sink
	log       *log.Logger
}

func New(providers []provider.Provider, sink Sink, logger *log.Logger) *Runner {
	return &Runner{
		providers: providers,
		sink:      sink,
		log:       logger.WithPrefix("runner"),
	}
}

func (r *Runner) PollAll(ctx context.Context) {
	if len(r.providers) == 0 {
		r.log.Info("no providers configured, skipping poll")
		return
	}

	for _, p := range r.providers {
		l := r.log.With("provider", p.Name())
		l.Info("polling")

		txs, err := p.Poll(ctx)
		if err != nil {
			// one failed provider shouldn't stop the others from being polled
			l.Error("poll failed", "err", err)
			continue
		}
		if len(txs) == 0 {
			l.Info("no new transactions")
			continue
		}

		l.Info("received transactions", "count", len(txs))
		if err := r.sink.CreateTransactions(ctx, txs); err != nil {
			l.Error("sink failed", "err", err, "dropped", len(txs))
			continue
		}
		l.Info("posted to null-core", "count", len(txs))
	}
}
