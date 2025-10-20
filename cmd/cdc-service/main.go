package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/database"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/elasticsearch"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/services/cdc"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Connect to database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Connect to Elasticsearch
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatal("Failed to connect to Elasticsearch:", err)
	}

	// Create service
	cdcService := cdc.NewService(db, esClient, cfg)

	// Setup Gin router
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Setup routes
	cdcService.SetupRoutes(r)

	// Start CDC worker in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cdcService.StartCDCWorker(ctx)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down CDC Service...")
		cancel()
	}()

	// Start server
	log.Printf("CDC Service starting on port %s", cfg.CDCServicePort)
	if err := r.Run(":" + cfg.CDCServicePort); err != nil {
		log.Fatal("Failed to start CDC Service:", err)
	}
}
