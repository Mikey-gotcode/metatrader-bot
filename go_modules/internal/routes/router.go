package routes

import (
	"net/http"
	"go_modules/internal/controllers"
)

func SetupRoutes(mux *http.ServeMux, marketCtrl *controllers.MarketController) {
	// Your existing trade logging route
	// mux.HandleFunc("/api/v1/trades/log", controllers.LogTrade)

	// New dynamic symbol route
	mux.HandleFunc("/api/v1/market/trending", marketCtrl.GetTrendingSymbols)
}
