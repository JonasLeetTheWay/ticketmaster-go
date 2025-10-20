package payment

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
)

type MockStripeClient struct {
	config *config.Config
}

type PaymentIntent struct {
	ID       string
	Amount   float64
	Currency string
	Status   string
}

type PaymentRequest struct {
	Amount   float64
	Currency string
	UserID   uint
	TicketID uint
}

type PaymentResponse struct {
	PaymentIntent *PaymentIntent
	Success       bool
	Error         string `json:"omitempty"`
}

func NewMockStripeClient(cfg *config.Config) *MockStripeClient {
	return &MockStripeClient{config: cfg}
}

func (c *MockStripeClient) CreatePaymentIntent(ctx context.Context, req *PaymentRequest) (*PaymentResponse, error) {
	// Simulate network delay
	time.Sleep(time.Duration(rand.Intn(500)+100) * time.Millisecond)

	// Generate mock payment intent ID
	paymentID := fmt.Sprintf("pi_mock_%d_%d", req.UserID, time.Now().Unix())

	// Determine success based on configured success rate
	success := rand.Float64() < c.config.MockStripeSuccessRate

	var status string
	var errMsg string

	if success {
		status = "succeeded"
	} else {
		status = "failed"
		errMsg = "Mock payment failure - insufficient funds"
	}

	paymentIntent := &PaymentIntent{
		ID:       paymentID,
		Amount:   req.Amount,
		Currency: req.Currency,
		Status:   status,
	}

	return &PaymentResponse{
		PaymentIntent: paymentIntent,
		Success:       success,
		Error:         errMsg,
	}, nil
}

func (c *MockStripeClient) ConfirmPaymentIntent(ctx context.Context, paymentIntentID string) (*PaymentResponse, error) {
	// Simulate network delay
	time.Sleep(time.Duration(rand.Intn(300)+50) * time.Millisecond)

	// For mock purposes, assume confirmation always succeeds if the intent exists
	// In a real implementation, you'd check the actual payment intent status

	return &PaymentResponse{
		PaymentIntent: &PaymentIntent{
			ID:       paymentIntentID,
			Status:   "succeeded",
			Currency: "ntd",
		},
		Success: true,
	}, nil
}

func (c *MockStripeClient) RefundPayment(ctx context.Context, paymentIntentID string, amount float64) (*PaymentResponse, error) {
	// Simulate network delay
	time.Sleep(time.Duration(rand.Intn(400)+100) * time.Millisecond)

	// Mock refund - always succeeds
	refundID := fmt.Sprintf("re_mock_%s_%d", paymentIntentID, time.Now().Unix())

	return &PaymentResponse{
		PaymentIntent: &PaymentIntent{
			ID:       refundID,
			Amount:   amount,
			Currency: "ntd",
			Status:   "succeeded",
		},
		Success: true,
	}, nil
}

func (c *MockStripeClient) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*PaymentIntent, error) {
	// Simulate network delay
	time.Sleep(time.Duration(rand.Intn(200)+50) * time.Millisecond)

	// Mock implementation - return a successful payment intent
	return &PaymentIntent{
		ID:       paymentIntentID,
		Status:   "succeeded",
		Currency: "ntd",
	}, nil
}
