package runner

import (
	"context"
	"time"

	"null-connector/internal/api"
	"null-connector/internal/domain"
	"null-connector/internal/provider"

	"github.com/charmbracelet/log"
)

type JobSource interface {
	ListSyncJobs(ctx context.Context) ([]api.SyncJob, error)
	CompleteSyncJob(ctx context.Context, id int64, cursor time.Time, status *string) error
}

// Sink receives transactions for delivery to null-core
type Sink interface {
	CreateTransactions(ctx context.Context, userID string, txs []domain.Transaction) error
}

type ProviderFactory func(api.SyncJob) (provider.Provider, error)

type Runner struct {
	jobs    JobSource
	sink    Sink
	factory ProviderFactory
	log     *log.Logger
}

func New(jobs JobSource, sink Sink, factory ProviderFactory, logger *log.Logger) *Runner {
	return &Runner{
		jobs:    jobs,
		sink:    sink,
		factory: factory,
		log:     logger.WithPrefix("runner"),
	}
}

func (r *Runner) RunOnce(ctx context.Context) {
	jobs, err := r.jobs.ListSyncJobs(ctx)
	if err != nil {
		r.log.Error("list sync jobs failed", "err", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	r.log.Info("running due jobs", "count", len(jobs))
	for _, job := range jobs {
		r.runJob(ctx, job)
	}
}

func (r *Runner) Run(ctx context.Context, interval time.Duration) {
	r.RunOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.RunOnce(ctx)
		}
	}
}

func (r *Runner) runJob(ctx context.Context, job api.SyncJob) {
	l := r.log.With("job_id", job.ID, "user_id", job.UserID, "provider", job.Provider)

	p, err := r.factory(job)
	if err != nil {
		l.Error("build provider failed", "err", err)
		return
	}

	l.Info("polling")
	txs, err := p.Poll(ctx)
	if err != nil {
		// TODO: detect auth-breaking errors and flip status to 'broken'
		l.Error("poll failed", "err", err)
		return
	}

	if len(txs) > 0 {
		if err := r.sink.CreateTransactions(ctx, job.UserID, txs); err != nil {
			l.Error("sink failed", "err", err, "dropped", len(txs))
			return
		}
		l.Info("posted to null-core", "count", len(txs))
	}

	if err := r.jobs.CompleteSyncJob(ctx, job.ID, time.Now(), nil); err != nil {
		l.Error("complete sync job failed", "err", err)
	}
}
