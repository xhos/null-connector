package domain

import "time"

type Direction int

const (
	DirectionUnknown Direction = iota
	DirectionIn
	DirectionOut
)

type Transaction struct {
	ExternalID string
	AccountID  int64
	Date       time.Time
	Amount     float64
	Currency   string
	Direction  Direction

	Description string
	Merchant    string
}
