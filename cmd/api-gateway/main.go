package main

import (
	"log"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/database"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/services/gateway"

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

	// Create service
	gatewayService := gateway.NewService(cfg, db)

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
	gatewayService.SetupRoutes(r)

	// Start server
	log.Printf("API Gateway starting on port %s", cfg.APIGatewayPort)
	if err := r.Run(":" + cfg.APIGatewayPort); err != nil {
		log.Fatal("Failed to start API Gateway:", err)
	}
}
