package event

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetupRoutes(r *gin.Engine) {
	r.GET("/event/:id", s.GetEvent)
	r.POST("/event", s.CreateEvent)
	r.PUT("/event/:id", s.UpdateEvent)
	r.DELETE("/event/:id", s.DeleteEvent)
	r.GET("/health", s.HealthCheck)
}

func (s *Service) GetEvent(c *gin.Context) {
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid event ID",
		})
		return
	}

	var event models.Event
	result := s.db.Preload("Venue").Preload("Performer").Preload("Tickets").First(&event, uint(eventID))
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Event not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch event",
			"details": result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, event)
}

func (s *Service) CreateEvent(c *gin.Context) {
	var event models.Event
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid event data",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if event.VenueID == 0 || event.PerformerID == 0 || event.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required fields: venueId, performerId, name",
		})
		return
	}

	// Check if venue exists
	var venue models.Venue
	if err := s.db.First(&venue, event.VenueID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Venue not found",
		})
		return
	}

	// Check if performer exists
	var performer models.Performer
	if err := s.db.First(&performer, event.PerformerID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Performer not found",
		})
		return
	}

	// Create event
	if err := s.db.Create(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create event",
			"details": err.Error(),
		})
		return
	}

	// Load relationships
	s.db.Preload("Venue").Preload("Performer").First(&event, event.ID)

	c.JSON(http.StatusCreated, event)
}

func (s *Service) UpdateEvent(c *gin.Context) {
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid event ID",
		})
		return
	}

	var event models.Event
	if err := s.db.First(&event, uint(eventID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Event not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch event",
			"details": err.Error(),
		})
		return
	}

	var updateData models.Event
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid event data",
			"details": err.Error(),
		})
		return
	}

	// Update event
	if err := s.db.Model(&event).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update event",
			"details": err.Error(),
		})
		return
	}

	// Load relationships
	s.db.Preload("Venue").Preload("Performer").Preload("Tickets").First(&event, event.ID)

	c.JSON(http.StatusOK, event)
}

func (s *Service) DeleteEvent(c *gin.Context) {
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid event ID",
		})
		return
	}

	// Check if event exists
	var event models.Event
	if err := s.db.First(&event, uint(eventID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Event not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch event",
			"details": err.Error(),
		})
		return
	}

	// Delete event (this will cascade to tickets due to foreign key constraints)
	if err := s.db.Delete(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete event",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Event deleted successfully",
	})
}

func (s *Service) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "event-service",
	})
}

// GetEventByID returns an event by ID with all relationships loaded
func (s *Service) GetEventByID(ctx context.Context, eventID uint) (*models.Event, error) {
	var event models.Event
	result := s.db.Preload("Venue").Preload("Performer").Preload("Tickets").First(&event, eventID)
	if result.Error != nil {
		return nil, result.Error
	}
	return &event, nil
}

// CreateTicketsForEvent creates tickets for an event
func (s *Service) CreateTicketsForEvent(ctx context.Context, eventID uint, ticketSpecs []TicketSpec) error {
	tickets := make([]models.Ticket, len(ticketSpecs))
	for i, spec := range ticketSpecs {
		tickets[i] = models.Ticket{
			EventID: eventID,
			Seat:    spec.Seat,
			Price:   spec.Price,
			Status:  "available",
		}
	}
	return s.db.Create(&tickets).Error
}

type TicketSpec struct {
	Seat  string  `json:"seat"`
	Price float64 `json:"price"`
}
