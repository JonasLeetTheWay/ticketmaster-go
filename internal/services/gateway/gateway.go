package gateway

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/auth"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Service struct {
	config *config.Config
	db     *gorm.DB
	client *http.Client
}

func NewService(cfg *config.Config, db *gorm.DB) *Service {
	return &Service{
		config: cfg,
		db:     db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Service) SetupRoutes(r *gin.Engine) {
	// Authentication routes
	r.POST("/auth/register", s.Register)
	r.POST("/auth/login", s.Login)

	// Search routes (forwarded to search service)
	r.GET("/search", s.ForwardToSearchService)

	// Event routes (forwarded to event service)
	r.GET("/event/:id", s.ForwardToEventService)

	// Booking routes (require authentication)
	booking := r.Group("/booking")
	booking.Use(s.AuthMiddleware())
	{
		booking.POST("/reserve", s.ForwardToBookingService)
		booking.PUT("/confirm", s.ForwardToBookingService)
		booking.DELETE("/cancel/:id", s.ForwardToBookingService)
		booking.GET("/user/:userId", s.ForwardToBookingService)
	}

	// Health check
	r.GET("/health", s.HealthCheck)
}

func (s *Service) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		Name     string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "User already exists",
		})
		return
	}

	// Hash password (simplified - in production use bcrypt)
	hashedPassword := hashPassword(req.Password)

	// Create user
	user := models.User{
		Email:    req.Email,
		Password: hashedPassword,
		Name:     req.Name,
	}

	if err := s.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create user",
		})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(s.config, user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate token",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

func (s *Service) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Find user
	var user models.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		return
	}

	// Verify password (simplified - in production use bcrypt)
	if !verifyPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(s.config, user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

func (s *Service) ForwardToSearchService(c *gin.Context) {
	s.forwardRequest(c, fmt.Sprintf("http://localhost:%s", s.config.SearchServicePort))
}

func (s *Service) ForwardToEventService(c *gin.Context) {
	s.forwardRequest(c, fmt.Sprintf("http://localhost:%s", s.config.EventServicePort))
}

func (s *Service) ForwardToBookingService(c *gin.Context) {
	s.forwardRequest(c, fmt.Sprintf("http://localhost:%s", s.config.BookingServicePort))
}

func (s *Service) forwardRequest(c *gin.Context, targetURL string) {
	// Create new request
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL+c.Request.URL.Path, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create request",
		})
		return
	}

	// Copy headers
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Add query parameters
	req.URL.RawQuery = c.Request.URL.RawQuery

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "Service unavailable",
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Copy response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read response",
		})
		return
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		tokenString, err := auth.ExtractTokenFromHeader(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header",
			})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(s.config, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Next()
	}
}

func (s *Service) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "api-gateway",
		"timestamp": time.Now(),
	})
}

// Simple password hashing (in production, use bcrypt)
func hashPassword(password string) string {
	// This is a simplified implementation
	// In production, use: bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return fmt.Sprintf("hashed_%s", password)
}

func verifyPassword(password, hashedPassword string) bool {
	// This is a simplified implementation
	// In production, use: bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return hashedPassword == fmt.Sprintf("hashed_%s", password)
}
