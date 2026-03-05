package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"go_modules/internal/controllers"
	"go_modules/internal/routes"
)

func main() {
	// 1. Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, relying on system environment variables")
	}

	// 2. Fetch the API key
	apiKey := os.Getenv("FINN_HUB_API_KEY")
	if apiKey == "" {
		log.Fatal("Fatal: FINNHUB_API_KEY is not set in .env file")
	}

	mux := http.NewServeMux()

	// Initialize Controllers
	marketCtrl := controllers.NewMarketController()
	
	// Start the background scanner using the secure API key
	marketCtrl.StartLiveScanner(apiKey)

	// Setup Routes
	routes.SetupRoutes(mux, marketCtrl)

	port := ":8080"
	fmt.Printf("Go Microservice starting on port %s...\n", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
