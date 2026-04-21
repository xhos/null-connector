package wise

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"null-connector/internal/api"
	"null-connector/internal/domain"
	wiseapi "null-connector/internal/wise"

	"github.com/charmbracelet/log"
)

type Config struct {
	APIToken string `json:"api_token"`
}

type Provider struct {
	wise   *wiseapi.Client
	core   *api.Client
	userID string
	cursor *time.Time
	log    *log.Logger
}

// wise does 469 days of history max, so we do a bit less just in case
const initialLookback = 468 * 24 * time.Hour

var brandColors = []string{"#163300", "#9FE870", "#D4F4B7"}

func New(cfg Config, core *api.Client, userID string, cursor *time.Time, logger *log.Logger) *Provider {
	return &Provider{
		wise:   wiseapi.NewClient(cfg.APIToken),
		core:   core,
		userID: userID,
		cursor: cursor,
		log:    logger.WithPrefix("wise"),
	}
}

func (p *Provider) Name() string { return "wise" }

func (p *Provider) Poll(ctx context.Context) ([]domain.Transaction, error) {
	profiles, err := p.wise.GetProfiles(ctx)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, errors.New("wise returned no profiles for this token")
	}
	profileID := profiles[0].ID

	balances, err := p.wise.ListBalances(ctx, profileID)
	if err != nil {
		return nil, err
	}
	p.log.Info("fetched balances", "count", len(balances))

	end := time.Now().UTC()
	start := end.Add(-initialLookback)
	if p.cursor != nil {
		start = p.cursor.UTC()
	}

	accountMap, err := p.buildAccountMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("build account map: %w", err)
	}

	var txs []domain.Transaction
	for _, b := range balances {
		accountID, err := p.resolveAccount(ctx, accountMap, b)
		if err != nil {
			p.log.Error("resolve account failed", "balance_id", b.ID, "currency", b.Currency, "err", err)
			continue
		}

		stmt, err := p.wise.GetStatement(ctx, profileID, b.ID, b.Currency, start, end)
		if err != nil {
			p.log.Error("get statement failed", "balance_id", b.ID, "currency", b.Currency, "err", err)
			continue
		}

		for i := range stmt.Transactions {
			txs = append(txs, toDomain(&stmt.Transactions[i], accountID))
		}
		p.log.Debug("balance polled", "balance_id", b.ID, "currency", b.Currency, "tx_count", len(stmt.Transactions))
	}

	return txs, nil
}

func (p *Provider) buildAccountMap(ctx context.Context) (map[string]int64, error) {
	accounts, err := p.core.ListAccounts(ctx, p.userID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(accounts))
	for _, a := range accounts {
		for _, alias := range a.Aliases {
			m[alias] = a.Id
		}
	}
	return m, nil
}

func (p *Provider) resolveAccount(ctx context.Context, accountMap map[string]int64, b wiseapi.Balance) (int64, error) {
	alias := fmt.Sprintf("wise-%d", b.ID)
	if id, ok := accountMap[alias]; ok {
		return id, nil
	}

	acc, err := p.core.CreateAccount(ctx, p.userID, "Wise "+b.Currency, "Wise", b.Currency, b.Amount.Value, brandColors)
	if err != nil {
		return 0, fmt.Errorf("create account: %w", err)
	}
	if err := p.core.AddAccountAlias(ctx, p.userID, acc.Id, alias); err != nil {
		p.log.Error("add alias failed, duplicate accounts may result next poll", "account_id", acc.Id, "alias", alias, "err", err)
	}
	accountMap[alias] = acc.Id
	p.log.Info("created account", "account_id", acc.Id, "balance_id", b.ID, "currency", b.Currency, "anchor", b.Amount.Value)
	return acc.Id, nil
}

func toDomain(w *wiseapi.Transaction, accountID int64) domain.Transaction {
	dir := domain.DirectionIn
	if w.Type == "DEBIT" {
		dir = domain.DirectionOut
	}

	amount := w.Amount.Value
	if amount < 0 {
		amount = -amount
	}

	tx := domain.Transaction{
		ExternalID:  w.ReferenceNumber,
		AccountID:   accountID,
		Date:        w.Date,
		Amount:      amount,
		Currency:    w.Amount.Currency,
		Direction:   dir,
		Description: w.Details.Description,
	}
	if w.Details.Merchant != nil && w.Details.Merchant.Name != "" {
		tx.Merchant = w.Details.Merchant.Name
	}

	if fee := w.TotalFees.Value; fee != 0 {
		totalCents := int64(math.Round(amount * 100))
		feeCents := int64(math.Round(fee * 100))
		tx.ReceiptItems = []domain.ReceiptItem{
			{Name: "Transfer", AmountCents: totalCents - feeCents},
			{Name: "Wise fee", AmountCents: feeCents},
		}
	}
	return tx
}
