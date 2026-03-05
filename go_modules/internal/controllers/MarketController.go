package controllers

import (
	"encoding/json"
	"go_modules/internal/models" // adjust to your actual module name
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type MarketController struct {
	mu              sync.RWMutex
	trendingSymbols map[string]models.TrendingSymbol
	tickVolume      map[string]int // <-- Added this field to track momentum
}

func NewMarketController() *MarketController {
	return &MarketController{
		trendingSymbols: make(map[string]models.TrendingSymbol),
		tickVolume:      make(map[string]int), // <-- Added map initialization
	}
}

// GetTrendingSymbols handles GET /api/v1/market/trending
func (mc *MarketController) GetTrendingSymbols(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var active []string
	// Filter logic: Only return symbols updated in the last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)

	for sym, data := range mc.trendingSymbols {
		if data.UpdatedAt.After(cutoff) {
			active = append(active, sym)
		}
	}

	// Fallback if the market is totally dead
	if len(active) == 0 {
		active = []string{"XAUUSD", "EURUSD"} // Safe defaults
	}

	response := models.MarketResponse{
		ActiveSymbols: active,
		Message:       "Success",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// --- REAL WEBSOCKET SCANNER ---

// FinnhubMessage represents the JSON structure sent by the Finnhub WebSocket
type FinnhubMessage struct {
	Data []struct {
		P float64 `json:"p"` // Last price
		S string  `json:"s"` // Symbol
		T int64   `json:"t"` // UNIX timestamp
		V float64 `json:"v"` // Volume
	} `json:"data"`
	Type string `json:"type"`
}

func (mc *MarketController) StartLiveScanner(apiKey string) {
	// 1. Connect to Finnhub WebSocket
	w, _, err := websocket.DefaultDialer.Dial("wss://ws.finnhub.io?token="+apiKey, nil)
	if err != nil {
		log.Fatalf("Fatal: Cannot connect to Finnhub WebSocket: %v", err)
	}
	log.Println("Successfully connected to Finnhub Market Stream!")

	// 2. Subscribe to the pairs you want to monitor
	// Note: Finnhub uses broker prefixes like OANDA:EUR_USD or BINANCE:BTCUSDT
	symbolsToTrack := []string{"OANDA:EUR_USD", "OANDA:GBP_JPY", "OANDA:XAU_USD", "OANDA:AUD_USD","BINANCE:BTCUSDT"}
	for _, sym := range symbolsToTrack {
		msg := map[string]string{"type": "subscribe", "symbol": sym}
		w.WriteJSON(msg)
	}

	// 3. Start listening for live market ticks
	go func() {
		defer w.Close()
		for {
			_, message, err := w.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				// In production, you'd want to implement reconnection logic here
				return 
			}

			var msg FinnhubMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}

			if msg.Type == "trade" {
				mc.processMarketTicks(msg)
			}
		}
	}()

	// 4. Background job to reset scores/volumes periodically
	go mc.decayMomentumScores()
}

func (mc *MarketController) processMarketTicks(msg FinnhubMessage) {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    for _, trade := range msg.Data {
        // Clean the symbol name for MT5
        cleanSymbol := strings.Replace(trade.S, "OANDA:", "", 1)
        cleanSymbol = strings.Replace(cleanSymbol, "BINANCE:", "", 1) // Clean Binance prefix too
        cleanSymbol = strings.Replace(cleanSymbol, "_", "", 1)

        // Increment tick volume (basic momentum tracking)
        mc.tickVolume[cleanSymbol]++

        // Update the trending map
        mc.trendingSymbols[cleanSymbol] = models.TrendingSymbol{
            Symbol:    cleanSymbol,
            Score:     float64(mc.tickVolume[cleanSymbol]),
            Session:   "DYNAMIC", 
            UpdatedAt: time.Now(),
        }

        // // ---> NEW LOGGING HERE <---
        // log.Printf("[LIVE TICK] %s | Price: %.5f | Current Momentum Score: %d", 
        //     cleanSymbol, trade.P, int(mc.trendingSymbols[cleanSymbol].Score))
    }
}

// decayMomentumScores resets the tick volume every minute so we only track *recent* momentum
func (mc *MarketController) decayMomentumScores() {
	for {
		time.Sleep(1 * time.Minute)
		mc.mu.Lock()
		// Reset tick volumes
		for k := range mc.tickVolume {
			mc.tickVolume[k] = 0
		}
		mc.mu.Unlock()
		log.Println("Market momentum scores decayed.")
	}
}

// LogTrade handles POST /api/v1/trades/log
func (mc *MarketController) LogTrade(w http.ResponseWriter, r *http.Request) {
	// 1. Ensure it's a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Decode the incoming JSON payload
	var tradeLog models.TradeLog
	if err := json.NewDecoder(r.Body).Decode(&tradeLog); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Printf("Failed to decode trade log: %v", err)
		return
	}
	defer r.Body.Close()

	// 3. Log the trade to the Go terminal
	log.Printf("💰 [TRADE EXECUTED] %s | %s | %s | Vol: %.2f | Profit: $%.2f",
		tradeLog.Timestamp.Format("15:04:05"),
		tradeLog.Symbol,
		tradeLog.Operation,
		tradeLog.Volume,
		tradeLog.Profit,
	)

	// 4. Send a success response back to Python
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Trade logged successfully",
	})
}