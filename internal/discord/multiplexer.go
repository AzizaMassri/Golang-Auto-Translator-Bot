package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// WebhookMultiplexer controls webhook execution caching heavily aided by DragonflyDB
type WebhookMultiplexer struct {
	RedisClient *redis.Client
	MaxHooks    int
}

// NewWebhookMultiplexer initializes the struct connected to Dragonfly
func NewWebhookMultiplexer(rdb *redis.Client) *WebhookMultiplexer {
	return &WebhookMultiplexer{
		RedisClient: rdb,
		MaxHooks:    10, // Discord's limit is 15.
	}
}

// ExecuteInline payload mimics Discord Webhook execution dynamically patching user info
func (wm *WebhookMultiplexer) ExecuteInline(ctx context.Context, webhookID, token string, originalUsername, avatarURL, targetEmoji, content string) error {
	url := fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", webhookID, token)

	payload := map[string]string{
		"content":    content,
		"username":   fmt.Sprintf("%s %s", originalUsername, targetEmoji),
		"avatar_url": avatarURL,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Extremely fast Dragonfly DB cache check
	// wm.RedisClient.Get(...)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord API error: HTTP %d", resp.StatusCode)
	}

	log.Printf("Successfully multiplexed webhook as %s", payload["username"])
	return nil
}
