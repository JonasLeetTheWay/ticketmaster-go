package main

import (
	"log"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/database"
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

	// Seed sample data
	if err := database.SeedData(db); err != nil {
		log.Fatal("Failed to seed data:", err)
	}

	log.Println("Database migration and seeding completed successfully")
}
