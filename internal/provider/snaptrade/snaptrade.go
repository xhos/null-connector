package snaptrade

import (
	"context"

	"null-connector/internal/domain"

	"github.com/charmbracelet/log"
)

type Config struct {
	// deployment-wide, populated by the factory from env vars
	ClientID    string `json:"-"`
	ConsumerKey string `json:"-"`
	// snaptrade's own per-end-user handle + secret, issued by /registerUser
	// and stored in the encrypted credentials blob. not the null-core UUID.
	SnapTradeUserID string `json:"snaptrade_user_id"`
	UserSecret      string `json:"user_secret"`
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
