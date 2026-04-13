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
		log.Printf("Worker picked up translation for %s [Enterprise: %v]", job.OriginalUsername, job.IsEnterprise)

		// 1. Translate via Gemini
		translatedText, err := gemini.Translate(ctx, job.Content, job.TargetLanguage, job.IsEnterprise)
		if err != nil {
			log.Printf("Gemini Translation failed: %v", err)
			return
		}

		// 2. Fetch/Recycle Webhook from DragonflyDB and Execute
		if botToken == "" {
			log.Println("Error: BOT_TOKEN missing. Cannot dispatch Webhook.")
			return
		}

		err = multiplexer.ExecuteInline(ctx, job.ChannelID, botToken, job.OriginalUsername, job.AvatarURL, "", translatedText)
		if err != nil {
			log.Printf("Multiplexer webhook execution failed: %v", err)
		}
	})

	if err != nil {
		log.Fatalf("Failed to start NATS consumer: %v", err)
	}
}
