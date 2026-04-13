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
}

// NewWebhookMultiplexer initializes the struct connected to Dragonfly
func NewWebhookMultiplexer(rdb *redis.Client) *WebhookMultiplexer {
	return &WebhookMultiplexer{RedisClient: rdb}
}

// Dispatch automatically fetches or creates a webhook for the channel, caches it, and maps the user avatar/name.
func (wm *WebhookMultiplexer) Dispatch(ctx context.Context, channelID, botToken, originalUsername, avatarURL, content string) error {
	// Forcing a v2 cache key to bypass any previously corrupted %!s(<nil>) tokens
	cacheKey := "webhook_v2:" + channelID
	
	// 1. Check cache for blazing fast O(1) execution
	cached, err := wm.RedisClient.Get(ctx, cacheKey).Result()
	var webhookURL string

	client := &http.Client{Timeout: 5 * time.Second}

	if err == redis.Nil || cached == "" {
		// Webhook not in cache, fetch or create it from Discord API
		url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/webhooks", channelID)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("Authorization", "Bot "+botToken)
		
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var webhooks []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&webhooks)

		for _, wh := range webhooks {
			if name, ok := wh["name"].(string); ok && name == "GlobalSync" {
				// Only accept it if Discord actually returned the token to us
				if token, ok := wh["token"].(string); ok && token != "" {
					webhookURL = fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", wh["id"], token)
					break
				}
			}
		}

		if webhookURL == "" {
			payload := map[string]string{"name": "GlobalSync"}
			body, _ := json.Marshal(payload)
			req, _ = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
			req.Header.Set("Authorization", "Bot "+botToken)
			req.Header.Set("Content-Type", "application/json")
			
			resp2, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp2.Body.Close()
			
			var newWH map[string]interface{}
			json.NewDecoder(resp2.Body).Decode(&newWH)
			if id, ok := newWH["id"].(string); ok {
				webhookURL = fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", id, newWH["token"])
			} else {
				return fmt.Errorf("failed to create webhook: %v", newWH)
			}
		}

		wm.RedisClient.Set(ctx, cacheKey, webhookURL, 0)
	} else {
		webhookURL = cached
	}

	// 2. Execute Spoofed Webhook
	payload := map[string]interface{}{
		"content":    content,
		"username":   originalUsername,
	}
	// Discord API rejects empty strings for Avatar URL
	if avatarURL != "" {
		payload["avatar_url"] = avatarURL
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord API error: HTTP %d", resp.StatusCode)
	}

	log.Printf("Successfully multiplexed webhook as [%s]", originalUsername)
	return nil
}
