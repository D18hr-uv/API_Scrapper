package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"api-scrapper/api"
	"api-scrapper/db"
	"api-scrapper/internal/embedder"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using raw environment variables.")
	}

	// 2. Initialize Database connection
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB()

	// 3. Initialize OpenAI Embedder
	if err := embedder.Init(); err != nil {
		log.Fatalf("Failed to initialize embedder: %v", err)
	}

	// 4. Initialize API Server
	app := fiber.New(fiber.Config{
		AppName: "API Scrapper Pipeline",
	})
	app.Use(logger.New())

	// Routes
	app.Post("/start-crawl", api.StartCrawlHandler)
	app.Get("/search", api.SearchHandler)
	app.Get("/graph", api.GraphHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Basic graceful shutdown setup
	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	log.Printf("Server is running on port %s", port)

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down cleanly...")
	app.Shutdown()
}
