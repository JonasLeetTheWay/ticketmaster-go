package main

import (
	"log"

	"ticketmaster-go/internal/config"
	"ticketmaster-go/internal/services/search"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Create service
	searchService, err := search.NewService(cfg)
	if err != nil {
		log.Fatal("Failed to create search service:", err)
	}

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
	searchService.SetupRoutes(r)

	// Start server
	log.Printf("Search Service starting on port %s", cfg.SearchServicePort)
	if err := r.Run(":" + cfg.SearchServicePort); err != nil {
		log.Fatal("Failed to start Search Service:", err)
	}
}
