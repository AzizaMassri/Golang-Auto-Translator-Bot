package worker

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"globalsync/internal/discord"
	"globalsync/internal/queue"
	"globalsync/internal/translator"
)

// StartTranslationWorker binds the NATS queue consumer to Gemini and Discord Webhooks.
func StartTranslationWorker(ctx context.Context, botToken string, natsMgr *queue.NatsManager, gemini *translator.GeminiService, multiplexer *discord.WebhookMultiplexer) {
	log.Println("Starting NATS consumer worker pool...")

	err := natsMgr.StartConsumer(ctx, func(job queue.TranslationJob) {
		log.Printf("Worker picked up batched translation for %s [Enterprise: %v]", job.OriginalUsername, job.IsEnterprise)

		if botToken == "" {
			log.Println("Error: BOT_TOKEN missing. Cannot dispatch Webhook.")
			return
		}

		// 1. String-Build the unified block
		finalText := job.Content + "\n\n"

		// 2. Translate via Gemini for each language intelligently bypassing API calls for generic jargon
		for _, lang := range job.TargetLanguages {
			hashBytes := md5.Sum([]byte(job.Content))
			hashStr := hex.EncodeToString(hashBytes[:])
			cacheKey := fmt.Sprintf("trans_cache:%s:%s", lang, hashStr)

			cachedText, err := multiplexer.RedisClient.Get(ctx, cacheKey).Result()
			var translatedText string

			if err == redis.Nil || cachedText == "" {
				// Cache Miss -> Hit Gemini API cost pipeline
				translatedText, err = gemini.Translate(ctx, job.Content, lang, job.IsEnterprise)
				if err != nil {
					log.Printf("Gemini Batched Translation failed for [%s]: %v", lang, err)
					continue
				}
				// Save to Cache Permanently
				multiplexer.RedisClient.Set(ctx, cacheKey, translatedText, 0)
			} else {
				log.Printf("Cache Hit! Bypassed API entirely for [%s]", cacheKey)
				translatedText = cachedText
			}

			finalText += translatedText + "\n"
		}

		// 3. Dispatch Unified Webhook Route
		err = multiplexer.Dispatch(ctx, job.ChannelID, botToken, job.OriginalUsername, job.AvatarURL, finalText)
		if err != nil {
			log.Printf("Multiplexer webhook execution failed: %v", err)
		}
	})

	if err != nil {
		log.Fatalf("Failed to start NATS consumer: %v", err)
	}
}
