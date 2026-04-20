package wise

import (
	"context"

	"null-connector/internal/domain"

	"github.com/charmbracelet/log"
)

type Config struct {
	APIToken string `json:"api_token"`
}

type Provider struct {
	cfg Config
	log *log.Logger
}

func New(cfg Config, logger *log.Logger) *Provider {
	return &Provider{
		cfg: cfg,
		log: logger.WithPrefix("wise"),
	}
}

func (p *Provider) Name() string { return "wise" }

func (p *Provider) Poll(ctx context.Context) ([]domain.Transaction, error) {
	p.log.Warn("poll not implemented")
	return nil, nil
}
