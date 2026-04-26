package snaptrade

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"null-connector/internal/api"
	"null-connector/internal/domain"

	"github.com/charmbracelet/log"
	sdk "github.com/passiv/snaptrade-sdks/sdks/go"
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
	cfg    Config
	core   *api.Client
	userID string
	cursor *time.Time
	log    *log.Logger
}

// SnapTrade typically retains ~2 years of history; pick a safe initial window.
const initialLookback = 730 * 24 * time.Hour

// Overlap the cursor window by a day to cover day-granularity trade_date
// timestamps on incremental polls. Dedup is handled by null-core on
// (account_id, external_id).
const cursorOverlap = 24 * time.Hour

const activitiesPageSize = 1000

var brandColors = []string{"#0A2540", "#00D4FF", "#7B61FF"}

func New(cfg Config, core *api.Client, userID string, cursor *time.Time, logger *log.Logger) *Provider {
	return &Provider{
		cfg:    cfg,
		core:   core,
		userID: userID,
		cursor: cursor,
		log:    logger.WithPrefix("snaptrade"),
	}
}

func (p *Provider) Name() string { return "snaptrade" }

func (p *Provider) Poll(ctx context.Context) ([]domain.Transaction, error) {
	client := p.apiClient(ctx)

	accounts, _, err := client.AccountInformationApi.
		ListUserAccounts(p.cfg.SnapTradeUserID, p.cfg.UserSecret).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("list snaptrade accounts: %w", err)
	}
	p.log.Info("fetched accounts", "count", len(accounts))

	end := time.Now().UTC()
	start := end.Add(-initialLookback)
	if p.cursor != nil {
		start = p.cursor.UTC().Add(-cursorOverlap)
	}
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	accountMap, err := p.buildAccountMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("build account map: %w", err)
	}

	var txs []domain.Transaction
	for i := range accounts {
		acc := &accounts[i]
		nullAccountID, err := p.resolveAccount(ctx, accountMap, acc)
		if err != nil {
			p.log.Error("resolve account failed", "snaptrade_account_id", acc.Id, "err", err)
			continue
		}

		activities, err := p.fetchActivities(ctx, client, acc.Id, startStr, endStr)
		if err != nil {
			p.log.Error("fetch activities failed", "snaptrade_account_id", acc.Id, "err", err)
			continue
		}

		for j := range activities {
			if tx, ok := toDomain(&activities[j], nullAccountID); ok {
				txs = append(txs, tx)
			}
		}
		p.log.Debug("account polled", "snaptrade_account_id", acc.Id, "activity_count", len(activities))
	}

	return txs, nil
}

func (p *Provider) apiClient(ctx context.Context) *sdk.APIClient {
	cfg := sdk.NewConfiguration()
	cfg.SetPartnerClientId(p.cfg.ClientID)
	cfg.SetConsumerKey(p.cfg.ConsumerKey)
	// Re-base onto the poll ctx so request cancellation propagates.
	apiKeys := cfg.Context.Value(sdk.ContextAPIKeys)
	cfg.Context = context.WithValue(ctx, sdk.ContextAPIKeys, apiKeys)
	cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	return sdk.NewAPIClient(cfg)
}

func (p *Provider) fetchActivities(ctx context.Context, client *sdk.APIClient, accountID, startDate, endDate string) ([]sdk.AccountUniversalActivity, error) {
	var all []sdk.AccountUniversalActivity
	offset := int32(0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req := client.AccountInformationApi.GetAccountActivities(accountID, p.cfg.SnapTradeUserID, p.cfg.UserSecret)
		req.StartDate(startDate)
		req.EndDate(endDate)
		req.Offset(offset)
		req.Limit(activitiesPageSize)
		page, _, err := req.Execute()
		if err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if len(page.Data) < activitiesPageSize {
			return all, nil
		}
		if page.Pagination != nil && page.Pagination.Total != nil && offset+int32(len(page.Data)) >= *page.Pagination.Total {
			return all, nil
		}
		offset += int32(len(page.Data))
	}
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

func (p *Provider) resolveAccount(ctx context.Context, accountMap map[string]int64, acc *sdk.Account) (int64, error) {
	alias := "snaptrade-" + acc.Id
	if id, ok := accountMap[alias]; ok {
		return id, nil
	}

	name := acc.Number
	if n := acc.Name.Get(); n != nil && *n != "" {
		name = *n
	}
	displayName := acc.InstitutionName
	if name != "" {
		displayName = acc.InstitutionName + " " + name
	}

	currency := "USD"
	var anchor float64
	if total := acc.Balance.Total.Get(); total != nil {
		if total.Currency != nil && *total.Currency != "" {
			currency = *total.Currency
		}
		if total.Amount != nil {
			anchor = float64(*total.Amount)
		}
	}

	created, err := p.core.CreateAccount(ctx, p.userID, displayName, acc.InstitutionName, currency, anchor, brandColors)
	if err != nil {
		return 0, fmt.Errorf("create account: %w", err)
	}
	if err := p.core.AddAccountAlias(ctx, p.userID, created.Id, alias); err != nil {
		p.log.Error("add alias failed, duplicate accounts may result next poll", "account_id", created.Id, "alias", alias, "err", err)
	}
	accountMap[alias] = created.Id
	p.log.Info("created account", "account_id", created.Id, "snaptrade_account_id", acc.Id, "currency", currency, "anchor", anchor)
	return created.Id, nil
}

func toDomain(a *sdk.AccountUniversalActivity, accountID int64) (domain.Transaction, bool) {
	if a.Id == nil || *a.Id == "" {
		return domain.Transaction{}, false
	}
	amtPtr := a.Amount.Get()
	if amtPtr == nil || *amtPtr == 0 {
		return domain.Transaction{}, false
	}
	amount := float64(*amtPtr)

	dir := domain.DirectionIn
	if amount < 0 {
		dir = domain.DirectionOut
		amount = -amount
	}

	currency := ""
	if a.Currency != nil && a.Currency.Code != nil {
		currency = *a.Currency.Code
	}

	var date time.Time
	if tPtr := a.TradeDate.Get(); tPtr != nil {
		date = *tPtr
	} else if a.SettlementDate != nil {
		date = *a.SettlementDate
	} else {
		date = time.Now().UTC()
	}

	description := ""
	if a.Description != nil {
		description = *a.Description
	}
	if a.Type != nil && *a.Type != "" {
		if description == "" {
			description = *a.Type
		} else {
			description = *a.Type + ": " + description
		}
	}

	tx := domain.Transaction{
		ExternalID:  *a.Id,
		AccountID:   accountID,
		Date:        date,
		Amount:      amount,
		Currency:    currency,
		Direction:   dir,
		Description: description,
	}

	if a.Fee != nil {
		fee := float64(*a.Fee)
		if fee < 0 {
			fee = -fee
		}
		if fee > 0 {
			totalCents := int64(math.Round(amount * 100))
			feeCents := int64(math.Round(fee * 100))
			label := "Activity"
			if a.Type != nil && *a.Type != "" {
				label = *a.Type
			}
			tx.ReceiptItems = []domain.ReceiptItem{
				{Name: label, AmountCents: totalCents - feeCents},
				{Name: "Fee", AmountCents: feeCents},
			}
		}
	}
	return tx, true
}
