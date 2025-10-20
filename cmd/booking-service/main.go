package main

import (
	"log"

	"ticketmaster-go/internal/config"
	"ticketmaster-go/internal/database"
	"ticketmaster-go/internal/redis"
	"ticketmaster-go/internal/services/booking"

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

	// Connect to Redis
	redisClient := redis.NewClient(cfg)

	// Create service
	bookingService := booking.NewService(db, redisClient, cfg)

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
	bookingService.SetupRoutes(r)

	// Start server
	log.Printf("Booking Service starting on port %s", cfg.BookingServicePort)
	if err := r.Run(":" + cfg.BookingServicePort); err != nil {
		log.Fatal("Failed to start Booking Service:", err)
	}
}
