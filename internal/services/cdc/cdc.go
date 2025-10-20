package cdc

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/elasticsearch"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db           *gorm.DB
	searchClient *elasticsearch.Client
	config       *config.Config
}

func NewService(db *gorm.DB, searchClient *elasticsearch.Client, cfg *config.Config) *Service {
	return &Service{
		db:           db,
		searchClient: searchClient,
		config:       cfg,
	}
}

func (s *Service) SetupRoutes(r *gin.Engine) {
	r.POST("/cdc/sync-event/:id", s.SyncEvent)
	r.POST("/cdc/sync-all", s.SyncAllEvents)
	r.GET("/health", s.HealthCheck)
}

func (s *Service) SyncEvent(c *gin.Context) {
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid event ID",
		})
		return
	}

	if err := s.syncEventByID(context.Background(), uint(eventID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to sync event",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Event synced successfully",
	})
}

func (s *Service) SyncAllEvents(c *gin.Context) {
	if err := s.syncAllEvents(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to sync all events",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "All events synced successfully",
	})
}

func (s *Service) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "cdc-service",
	})
}

// syncEventByID syncs a specific event to Elasticsearch
func (s *Service) syncEventByID(ctx context.Context, eventID uint) error {
	var event models.Event
	result := s.db.Preload("Venue").Preload("Performer").Preload("Tickets").First(&event, eventID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Event was deleted, remove from Elasticsearch
			return s.searchClient.DeleteEvent(eventID)
		}
		return fmt.Errorf("failed to fetch event: %w", result.Error)
	}

	// Convert to Elasticsearch document
	esEvent := s.convertToElasticsearchEvent(&event)

	// Index in Elasticsearch
	return s.searchClient.IndexEvent(esEvent)
}

// syncAllEvents syncs all events to Elasticsearch
func (s *Service) syncAllEvents(ctx context.Context) error {
	var events []models.Event
	result := s.db.Preload("Venue").Preload("Performer").Preload("Tickets").Find(&events)
	if result.Error != nil {
		return fmt.Errorf("failed to fetch events: %w", result.Error)
	}

	for _, event := range events {
		esEvent := s.convertToElasticsearchEvent(&event)
		if err := s.searchClient.IndexEvent(esEvent); err != nil {
			log.Printf("Failed to sync event %d: %v", event.ID, err)
			continue
		}
	}

	return nil
}

// convertToElasticsearchEvent converts a database event to Elasticsearch document
func (s *Service) convertToElasticsearchEvent(event *models.Event) *models.ElasticsearchEvent {
	esEvent := &models.ElasticsearchEvent{
		ID:          event.ID,
		VenueID:     event.VenueID,
		PerformerID: event.PerformerID,
		Name:        event.Name,
		Description: event.Description,
		Date:        event.Date.Format("2006-01-02T15:04:05Z"),
		Venue:       event.Venue.Location,
		Performer:   event.Performer.Name,
		Genre:       event.Performer.Genre,
		Location:    event.Venue.Location,
	}

	// Calculate price range and available tickets
	if len(event.Tickets) > 0 {
		minPrice := event.Tickets[0].Price
		maxPrice := event.Tickets[0].Price
		availableCount := 0

		for _, ticket := range event.Tickets {
			if ticket.Price < minPrice {
				minPrice = ticket.Price
			}
			if ticket.Price > maxPrice {
				maxPrice = ticket.Price
			}
			if ticket.Status == "available" {
				availableCount++
			}
		}

		esEvent.MinPrice = minPrice
		esEvent.MaxPrice = maxPrice
		esEvent.AvailableTickets = availableCount
	}

	return esEvent
}

// StartCDCWorker starts a background worker that periodically syncs changes
func (s *Service) StartCDCWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Sync every 30 seconds
	defer ticker.Stop()

	log.Println("CDC worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("CDC worker stopped")
			return
		case <-ticker.C:
			if err := s.syncRecentChanges(ctx); err != nil {
				log.Printf("CDC sync error: %v", err)
			}
		}
	}
}

// syncRecentChanges syncs events that have been modified recently
func (s *Service) syncRecentChanges(ctx context.Context) error {
	// Get events modified in the last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)

	var events []models.Event
	result := s.db.Preload("Venue").Preload("Performer").Preload("Tickets").
		Where("updated_at > ?", cutoff).Find(&events)
	if result.Error != nil {
		return fmt.Errorf("failed to fetch recent events: %w", result.Error)
	}

	for _, event := range events {
		esEvent := s.convertToElasticsearchEvent(&event)
		if err := s.searchClient.IndexEvent(esEvent); err != nil {
			log.Printf("Failed to sync recent event %d: %v", event.ID, err)
			continue
		}
	}

	if len(events) > 0 {
		log.Printf("Synced %d recent events", len(events))
	}

	return nil
}
