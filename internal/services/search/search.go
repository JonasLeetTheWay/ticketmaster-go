package search

import (
	"context"
	"fmt"
	"net/http"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/elasticsearch"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"

	"github.com/gin-gonic/gin"
)

type Service struct {
	esClient *elasticsearch.Client
}

func NewService(cfg *config.Config) (*Service, error) {
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	return &Service{
		esClient: esClient,
	}, nil
}

func (s *Service) SetupRoutes(r *gin.Engine) {
	r.GET("/search", s.SearchEvents)
	r.GET("/health", s.HealthCheck)
}

func (s *Service) SearchEvents(c *gin.Context) {
	// Extract query parameters
	term := c.Query("term")
	location := c.Query("location")
	eventType := c.Query("type")
	date := c.Query("date")

	// Build Elasticsearch query
	query := elasticsearch.BuildSearchQuery(term, location, eventType, date)

	// Execute search
	events, err := s.esClient.SearchEvents(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to search events",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

func (s *Service) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "search-service",
	})
}

// IndexEvent indexes an event in Elasticsearch
func (s *Service) IndexEvent(ctx context.Context, event *models.Event) error {
	// Convert to Elasticsearch document
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

	return s.esClient.IndexEvent(esEvent)
}

// UpdateEvent updates an event in Elasticsearch
func (s *Service) UpdateEvent(ctx context.Context, event *models.Event) error {
	return s.IndexEvent(ctx, event) // Elasticsearch treats update as index
}

// DeleteEvent removes an event from Elasticsearch
func (s *Service) DeleteEvent(ctx context.Context, eventID uint) error {
	return s.esClient.DeleteEvent(eventID)
}
