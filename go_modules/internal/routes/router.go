package routes

import (
	"net/http"
	"go_modules/internal/controllers"
)

func SetupRoutes(mux *http.ServeMux, marketCtrl *controllers.MarketController) {
	// Your existing trade logging route
	mux.HandleFunc("/api/v1/trades/log", marketCtrl.LogTrade)
	//mux.HandleFunc("/api/v1/market/", marketCtrl.GetTrendingSymbols)

	// New dynamic symbol route
	mux.HandleFunc("/api/v1/market/trending", marketCtrl.GetTrendingSymbols)
}
