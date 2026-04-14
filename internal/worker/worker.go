package worker

import (
	"context"
	"log"

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

		// 2. Translate via Gemini for each language
		for _, lang := range job.TargetLanguages {
			translatedText, err := gemini.Translate(ctx, job.Content, lang, job.IsEnterprise)
			if err != nil {
				log.Printf("Gemini Batched Translation failed for [%s]: %v", lang, err)
				continue
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
