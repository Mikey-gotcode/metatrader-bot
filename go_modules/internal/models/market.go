package models

import "time"

// TrendingSymbol represents a market pair that currently has high momentum or volume.
type TrendingSymbol struct {
	Symbol    string    `json:"symbol"`
	Score     float64   `json:"score"`      // Could be volume spike %, RSI, etc.
	Session   string    `json:"session"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MarketResponse is the JSON structure sent back to Python
type MarketResponse struct {
	ActiveSymbols []string `json:"active_symbols"`
	Message       string   `json:"message"`
}
