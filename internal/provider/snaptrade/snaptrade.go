package snaptrade

import (
	"context"

	"null-connector/internal/domain"

	"github.com/charmbracelet/log"
)

type Config struct {
	ClientID    string
	ConsumerKey string
}

type Provider struct {
	cfg Config
	log *log.Logger
}

func New(cfg Config, logger *log.Logger) *Provider {
	return &Provider{
		cfg: cfg,
		log: logger.WithPrefix("snaptrade"),
	}
}

func (p *Provider) Name() string { return "snaptrade" }

func (p *Provider) Poll(ctx context.Context) ([]domain.Transaction, error) {
	p.log.Warn("poll not implemented")
	return nil, nil
}
