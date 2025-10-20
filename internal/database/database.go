package database

import (
	"fmt"
	"log"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	if err := models.Migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database connected and migrated successfully")
	return db, nil
}

func SeedData(db *gorm.DB) error {
	// Check if data already exists
	var count int64
	db.Model(&models.Venue{}).Count(&count)
	if count > 0 {
		log.Println("Data already seeded, skipping...")
		return nil
	}

	// Create venues
	venues := []models.Venue{
		{Location: "Madison Square Garden, New York", SeatMap: `{"sections": ["A", "B", "C"], "rows": 50, "seatsPerRow": 20}`, Capacity: 20789},
		{Location: "Hollywood Bowl, Los Angeles", SeatMap: `{"sections": ["1", "2", "3"], "rows": 30, "seatsPerRow": 25}`, Capacity: 17500},
		{Location: "Royal Albert Hall, London", SeatMap: `{"sections": ["Stalls", "Circle", "Gallery"], "rows": 40, "seatsPerRow": 15}`, Capacity: 5272},
	}

	for _, venue := range venues {
		if err := db.Create(&venue).Error; err != nil {
			return fmt.Errorf("failed to create venue: %w", err)
		}
	}

	// Create performers
	performers := []models.Performer{
		{Name: "Taylor Swift", Description: "Pop superstar", Genre: "Pop"},
		{Name: "Coldplay", Description: "British rock band", Genre: "Rock"},
		{Name: "Ed Sheeran", Description: "Singer-songwriter", Genre: "Pop"},
		{Name: "Billie Eilish", Description: "Alternative pop artist", Genre: "Alternative"},
	}

	for _, performer := range performers {
		if err := db.Create(&performer).Error; err != nil {
			return fmt.Errorf("failed to create performer: %w", err)
		}
	}

	// Create events
	events := []models.Event{
		{VenueID: 1, PerformerID: 1, Name: "Taylor Swift - Eras Tour", Description: "The Eras Tour is coming to Madison Square Garden", Date: parseDate("2024-06-15T20:00:00Z")},
		{VenueID: 2, PerformerID: 2, Name: "Coldplay - Music of the Spheres", Description: "Experience Coldplay's cosmic journey", Date: parseDate("2024-07-20T19:30:00Z")},
		{VenueID: 3, PerformerID: 3, Name: "Ed Sheeran - Mathematics Tour", Description: "Ed Sheeran's intimate acoustic performance", Date: parseDate("2024-08-10T20:00:00Z")},
		{VenueID: 1, PerformerID: 4, Name: "Billie Eilish - Happier Than Ever", Description: "Billie Eilish's hauntingly beautiful performance", Date: parseDate("2024-09-05T19:00:00Z")},
	}

	var createdEvents []models.Event
	for _, event := range events {
		if err := db.Create(&event).Error; err != nil {
			return fmt.Errorf("failed to create event: %w", err)
		}
		createdEvents = append(createdEvents, event)
	}

	// Create tickets for each event
	for _, event := range createdEvents {
		if err := createTicketsForEvent(db, event.ID, event.VenueID); err != nil {
			return fmt.Errorf("failed to create tickets for event %d: %w", event.ID, err)
		}
	}

	log.Println("Sample data seeded successfully")
	return nil
}

func createTicketsForEvent(db *gorm.DB, eventID, venueID uint) error {
	var venue models.Venue
	if err := db.First(&venue, venueID).Error; err != nil {
		return err
	}

	// Create tickets with different price tiers
	priceTiers := []struct {
		section string
		price   float64
		count   int
	}{
		{"VIP", 299.99, 100},
		{"Premium", 199.99, 200},
		{"Standard", 99.99, 300},
		{"Economy", 49.99, 400},
	}

	for _, tier := range priceTiers {
		for i := 0; i < tier.count; i++ {
			ticket := models.Ticket{
				EventID: eventID,
				Seat:    fmt.Sprintf("%s-%d", tier.section, i+1),
				Price:   tier.price,
				Status:  "available",
			}
			if err := db.Create(&ticket).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func parseDate(dateStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, dateStr)
	return t
}
