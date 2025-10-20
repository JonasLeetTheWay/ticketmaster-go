package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/go-redis/redis/v8"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(cfg *config.Config) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// LockTicket reserves a ticket for 10 minutes
func (c *Client) LockTicket(ctx context.Context, ticketID uint, userID uint) error {
	key := fmt.Sprintf("ticket_lock:%d", ticketID)
	value := fmt.Sprintf("%d", userID)

	// Try to set the lock with expiration
	result := c.rdb.SetNX(ctx, key, value, 10*time.Minute)
	if result.Err() != nil {
		return fmt.Errorf("failed to lock ticket: %w", result.Err())
	}

	if !result.Val() {
		return fmt.Errorf("ticket %d is already locked", ticketID)
	}

	return nil
}

// UnlockTicket releases a ticket lock
func (c *Client) UnlockTicket(ctx context.Context, ticketID uint) error {
	key := fmt.Sprintf("ticket_lock:%d", ticketID)
	return c.rdb.Del(ctx, key).Err()
}

// IsTicketLocked checks if a ticket is currently locked
func (c *Client) IsTicketLocked(ctx context.Context, ticketID uint) (bool, error) {
	key := fmt.Sprintf("ticket_lock:%d", ticketID)
	result := c.rdb.Exists(ctx, key)
	if result.Err() != nil {
		return false, result.Err()
	}
	return result.Val() > 0, nil
}

// GetTicketLockOwner returns the user ID who locked the ticket
func (c *Client) GetTicketLockOwner(ctx context.Context, ticketID uint) (uint, error) {
	key := fmt.Sprintf("ticket_lock:%d", ticketID)
	result := c.rdb.Get(ctx, key)
	if result.Err() != nil {
		return 0, result.Err()
	}

	var userID uint
	if _, err := fmt.Sscanf(result.Val(), "%d", &userID); err != nil {
		return 0, fmt.Errorf("invalid user ID in lock: %w", err)
	}

	return userID, nil
}

// ExtendTicketLock extends the lock expiration by 10 minutes
func (c *Client) ExtendTicketLock(ctx context.Context, ticketID uint) error {
	key := fmt.Sprintf("ticket_lock:%d", ticketID)
	return c.rdb.Expire(ctx, key, 10*time.Minute).Err()
}

// AddToWaitingQueue adds a user to the virtual waiting queue
func (c *Client) AddToWaitingQueue(ctx context.Context, eventID uint, userID uint) error {
	key := fmt.Sprintf("waiting_queue:%d", eventID)
	return c.rdb.LPush(ctx, key, fmt.Sprintf("%d", userID)).Err()
}

// GetWaitingQueuePosition returns the position of a user in the waiting queue
func (c *Client) GetWaitingQueuePosition(ctx context.Context, eventID uint, userID uint) (int64, error) {
	key := fmt.Sprintf("waiting_queue:%d", eventID)
	result := c.rdb.LPos(ctx, key, fmt.Sprintf("%d", userID), redis.LPosArgs{})
	if result.Err() != nil {
		return -1, result.Err()
	}
	return result.Val(), nil
}

// ProcessWaitingQueue processes the next user in the waiting queue
func (c *Client) ProcessWaitingQueue(ctx context.Context, eventID uint) (uint, error) {
	key := fmt.Sprintf("waiting_queue:%d", eventID)
	result := c.rdb.RPop(ctx, key)
	if result.Err() != nil {
		return 0, result.Err()
	}

	var userID uint
	if _, err := fmt.Sscanf(result.Val(), "%d", &userID); err != nil {
		return 0, fmt.Errorf("invalid user ID in queue: %w", err)
	}

	return userID, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}
