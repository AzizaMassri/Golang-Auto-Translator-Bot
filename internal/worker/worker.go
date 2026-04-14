package worker

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

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

		// 2. Translate via Gemini entirely parallelized using Goroutines
		var wg sync.WaitGroup
		var mu sync.Mutex
		translations := make([]string, len(job.TargetLanguages))

		for i, lang := range job.TargetLanguages {
			wg.Add(1)
			go func(idx int, targetLang string) {
				defer wg.Done()

				hashBytes := md5.Sum([]byte(job.Content))
				hashStr := hex.EncodeToString(hashBytes[:])
				cacheKey := fmt.Sprintf("trans_cache:%s:%s", targetLang, hashStr)

				cachedText, err := multiplexer.RedisClient.Get(ctx, cacheKey).Result()
				var translatedText string

				if err == redis.Nil || cachedText == "" {
					// Cache Miss -> Hit Gemini API natively
					translatedText, err = gemini.Translate(ctx, job.Content, targetLang, job.IsEnterprise)
					if err != nil {
						log.Printf("Gemini Batched Translation failed for [%s]: %v", targetLang, err)
						return
					}
					// Save strictly to DB memory
					multiplexer.RedisClient.Set(ctx, cacheKey, translatedText, 0)
				} else {
					log.Printf("Cache Hit! Bypassed API entirely for [%s]", cacheKey)
					translatedText = cachedText
				}

				mu.Lock()
				translations[idx] = translatedText
				mu.Unlock()
			}(i, lang)
		}
		
		// Unblock completely once the thread-pool resolves
		wg.Wait()

		// Systematically string-build into the unified final format sequentially
		for _, t := range translations {
			if t != "" {
				finalText += t + "\n"
			}
		}

		// 3. Dispatch Unified Webhook Route
		err := multiplexer.Dispatch(ctx, job.ChannelID, botToken, job.OriginalUsername, job.AvatarURL, finalText)
		if err != nil {
			log.Printf("Multiplexer webhook execution failed: %v", err)
		}
	})

	if err != nil {
		log.Fatalf("Failed to start NATS consumer: %v", err)
	}
}
