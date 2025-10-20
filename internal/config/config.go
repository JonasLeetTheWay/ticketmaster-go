package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// Elasticsearch
	ElasticsearchURL string

	// JWT
	JWTSecret string
	JWTExpiry time.Duration

	// Service Ports
	APIGatewayPort     string
	SearchServicePort  string
	EventServicePort   string
	BookingServicePort string
	CDCServicePort     string

	// Mock Stripe
	MockStripeEnabled     bool
	MockStripeSuccessRate float64
}

func Load() (*Config, error) {
	godotenv.Load()
	
	config := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "ticketmaster"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		ElasticsearchURL: getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),

		JWTSecret: getEnv("JWT_SECRET", "your-secret-key-here"),
		JWTExpiry: parseDuration(getEnv("JWT_EXPIRY", "24h")),

		APIGatewayPort:     getEnv("API_GATEWAY_PORT", "8080"),
		SearchServicePort:  getEnv("SEARCH_SERVICE_PORT", "8081"),
		EventServicePort:   getEnv("EVENT_SERVICE_PORT", "8082"),
		BookingServicePort: getEnv("BOOKING_SERVICE_PORT", "8083"),
		CDCServicePort:     getEnv("CDC_SERVICE_PORT", "8084"),

		MockStripeEnabled:     getEnvBool("MOCK_STRIPE_ENABLED", true),
		MockStripeSuccessRate: getEnvFloat("MOCK_STRIPE_SUCCESS_RATE", 0.95),
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	valueBool, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	} else {
		return valueBool
	}
}

func getEnvFloat(key string, defaultValue float64) float64{
	value := os.Getenv(key)
	valueBool, err := strconv.ParseFloat(value, 64) // 64 is bitsize
	if err != nil {
		return defaultValue
	} else {
		return valueBool
	}
}

func parseDuration(s string) time.Duration {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return 24 * time.Hour // default 24 hr
	}
	return duration
}