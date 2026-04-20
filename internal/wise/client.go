package wise

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://api.wise.com"

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

type Profile struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "personal" | "business"
}

type Balance struct {
	ID       int64  `json:"id"`
	Currency string `json:"currency"`
	Amount   Amount `json:"amount"`
	Type     string `json:"type"`
	Primary  bool   `json:"primary"`
	Visible  bool   `json:"visible"`
}

type Amount struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
	Zero     bool    `json:"zero"`
}

type Merchant struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	Country  string `json:"country"`
	Category string `json:"category"`
}

// we only unmarshal the fields we care about, the rest is ignored
type Details struct {
	Type        string    `json:"type"` // CARD | CONVERSION | MONEY_ADDED | ...
	Description string    `json:"description"`
	Merchant    *Merchant `json:"merchant,omitempty"`
}

type Transaction struct {
	Type            string    `json:"type"` // DEBIT | CREDIT
	Date            time.Time `json:"date"`
	Amount          Amount    `json:"amount"`
	TotalFees       Amount    `json:"totalFees"`
	Details         Details   `json:"details"`
	ReferenceNumber string    `json:"referenceNumber"`
}

type Statement struct {
	Transactions []Transaction `json:"transactions"`
}

func (c *Client) GetProfiles(ctx context.Context) ([]Profile, error) {
	var out []Profile
	if err := c.get(ctx, "/v1/profiles", nil, &out); err != nil {
		return nil, fmt.Errorf("get profiles: %w", err)
	}
	return out, nil
}

func (c *Client) ListBalances(ctx context.Context, profileID int64) ([]Balance, error) {
	path := fmt.Sprintf("/v4/profiles/%d/balances", profileID)
	q := url.Values{"types": {"STANDARD"}}
	var out []Balance
	if err := c.get(ctx, path, q, &out); err != nil {
		return nil, fmt.Errorf("list balances: %w", err)
	}
	return out, nil
}

func (c *Client) GetStatement(ctx context.Context, profileID, balanceID int64, currency string, start, end time.Time) (*Statement, error) {
	path := fmt.Sprintf("/v1/profiles/%d/balance-statements/%d/statement.json", profileID, balanceID)
	q := url.Values{
		"currency":      {currency},
		"intervalStart": {start.UTC().Format("2006-01-02T15:04:05.000Z")},
		"intervalEnd":   {end.UTC().Format("2006-01-02T15:04:05.000Z")},
		"type":          {"COMPACT"},
	}
	var out Statement
	if err := c.get(ctx, path, q, &out); err != nil {
		return nil, fmt.Errorf("get statement: %w", err)
	}
	return &out, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wise %s %s: http %d: %s", http.MethodGet, path, resp.StatusCode, truncate(string(body), 500))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
