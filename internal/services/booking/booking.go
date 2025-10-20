package booking

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/auth"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/payment"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/redis"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	db            *gorm.DB
	redisClient   *redis.Client
	paymentClient *payment.MockStripeClient
	config        *config.Config
}

func NewService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *Service {
	return &Service{
		db:            db,
		redisClient:   redisClient,
		paymentClient: payment.NewMockStripeClient(cfg),
		config:        cfg,
	}
}

func (s *Service) SetupRoutes(r *gin.Engine) {
	r.POST("/booking/reserve", s.ReserveTicket)
	r.PUT("/booking/confirm", s.ConfirmBooking)
	r.DELETE("/booking/cancel/:id", s.CancelBooking)
	r.GET("/booking/user/:userId", s.GetUserBookings)
	r.GET("/health", s.HealthCheck)
}

func (s *Service) ReserveTicket(c *gin.Context) {
	// Extract and validate JWT token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	tokenString, err := auth.ExtractTokenFromHeader(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header",
		})
		return
	}

	claims, err := auth.ValidateToken(s.config, tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
		})
		return
	}

	// Parse request body
	var req struct {
		TicketID uint `json:"ticketId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Check if ticket exists and is available
	var ticket models.Ticket
	result := s.db.Preload("Event").First(&ticket, req.TicketID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Ticket not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch ticket",
		})
		return
	}

	if ticket.Status != "available" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Ticket is not available",
		})
		return
	}

	// Try to lock the ticket in Redis
	if err := s.redisClient.LockTicket(context.Background(), req.TicketID, claims.UserID); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Ticket is currently being processed by another user",
		})
		return
	}

	// Create booking record
	booking := models.Booking{
		TicketID:   req.TicketID,
		UserID:     claims.UserID,
		Status:     "reserved",
		ReservedAt: time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute), // 10 minute reservation window
	}

	if err := s.db.Create(&booking).Error; err != nil {
		// Release the lock if database operation fails
		s.redisClient.UnlockTicket(context.Background(), req.TicketID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create booking",
		})
		return
	}

	// Update ticket status
	s.db.Model(&ticket).Update("status", "reserved")

	c.JSON(http.StatusOK, gin.H{
		"bookingId": booking.ID,
		"ticketId":  req.TicketID,
		"expiresAt": booking.ExpiresAt,
		"message":   "Ticket reserved successfully",
	})
}

func (s *Service) ConfirmBooking(c *gin.Context) {
	// Extract and validate JWT token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	tokenString, err := auth.ExtractTokenFromHeader(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header",
		})
		return
	}

	claims, err := auth.ValidateToken(s.config, tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
		})
		return
	}

	// Parse request body
	var req struct {
		TicketID       uint   `json:"ticketId" binding:"required"`
		PaymentDetails string `json:"paymentDetails" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Check if booking exists and belongs to user
	var booking models.Booking
	result := s.db.Preload("Ticket").Preload("Ticket.Event").First(&booking, "ticket_id = ? AND user_id = ? AND status = ?", req.TicketID, claims.UserID, "reserved")
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "No active reservation found for this ticket",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch booking",
		})
		return
	}

	// Check if reservation has expired
	if time.Now().After(booking.ExpiresAt) {
		// Clean up expired booking
		s.db.Delete(&booking)
		s.redisClient.UnlockTicket(context.Background(), req.TicketID)
		s.db.Model(&models.Ticket{ID: req.TicketID}).Update("status", "available")

		c.JSON(http.StatusGone, gin.H{
			"error": "Reservation has expired",
		})
		return
	}

	// Verify the ticket is still locked by this user
	lockOwner, err := s.redisClient.GetTicketLockOwner(context.Background(), req.TicketID)
	if err != nil || lockOwner != claims.UserID {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Ticket lock has been released",
		})
		return
	}

	// Process payment
	paymentReq := &payment.PaymentRequest{
		Amount:   booking.Ticket.Price,
		Currency: "ntd",
		UserID:   claims.UserID,
		TicketID: req.TicketID,
	}

	paymentResp, err := s.paymentClient.CreatePaymentIntent(context.Background(), paymentReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Payment processing failed",
		})
		return
	}

	if !paymentResp.Success {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":   "Payment failed",
			"details": paymentResp.Error,
		})
		return
	}

	// Start database transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update booking status
	if err := tx.Model(&booking).Updates(map[string]interface{}{
		"status":     "confirmed",
		"payment_id": paymentResp.PaymentIntent.ID,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to confirm booking",
		})
		return
	}

	// Update ticket status and assign to user
	if err := tx.Model(&booking.Ticket).Updates(map[string]interface{}{
		"status":  "booked",
		"user_id": claims.UserID,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update ticket",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to confirm booking",
		})
		return
	}

	// Release Redis lock
	s.redisClient.UnlockTicket(context.Background(), req.TicketID)

	c.JSON(http.StatusOK, gin.H{
		"bookingId": booking.ID,
		"ticketId":  req.TicketID,
		"paymentId": paymentResp.PaymentIntent.ID,
		"message":   "Booking confirmed successfully",
	})
}

func (s *Service) CancelBooking(c *gin.Context) {
	bookingIDStr := c.Param("id")
	bookingID, err := strconv.ParseUint(bookingIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid booking ID",
		})
		return
	}

	// Extract and validate JWT token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	tokenString, err := auth.ExtractTokenFromHeader(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header",
		})
		return
	}

	claims, err := auth.ValidateToken(s.config, tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
		})
		return
	}

	// Check if booking exists and belongs to user
	var booking models.Booking
	result := s.db.Preload("Ticket").First(&booking, "id = ? AND user_id = ?", uint(bookingID), claims.UserID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Booking not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch booking",
		})
		return
	}

	// Start database transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update booking status
	if err := tx.Model(&booking).Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cancel booking",
		})
		return
	}

	// Update ticket status back to available
	if err := tx.Model(&booking.Ticket).Updates(map[string]interface{}{
		"status":  "available",
		"user_id": nil,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update ticket",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cancel booking",
		})
		return
	}

	// Release Redis lock if it exists
	s.redisClient.UnlockTicket(context.Background(), booking.TicketID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Booking cancelled successfully",
	})
}

func (s *Service) GetUserBookings(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	// Extract and validate JWT token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	tokenString, err := auth.ExtractTokenFromHeader(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header",
		})
		return
	}

	claims, err := auth.ValidateToken(s.config, tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
		})
		return
	}

	// Verify user can only access their own bookings
	if claims.UserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	var bookings []models.Booking
	result := s.db.Preload("Ticket").Preload("Ticket.Event").Preload("Ticket.Event.Venue").Preload("Ticket.Event.Performer").Find(&bookings, "user_id = ?", uint(userID))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch bookings",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings": bookings,
		"count":    len(bookings),
	})
}

func (s *Service) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "booking-service",
	})
}
