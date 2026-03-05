package models

import "time"

// TradeLog represents the data sent from the Python MT5 bot
type TradeLog struct {
	UserID    string    `json:"user_id"`
	Symbol    string    `json:"symbol"`
	Operation string    `json:"operation"`
	Volume    float64   `json:"volume"`
	Profit    float64   `json:"profit"`
	Timestamp time.Time `json:"timestamp"` // Go's json package automatically parses ISO 8601 strings into time.Time
}