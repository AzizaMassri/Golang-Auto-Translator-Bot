package worker

import (
	"context"
	"log"

	"globalsync/internal/discord"
	"globalsync/internal/queue"
	"globalsync/internal/translator"
)

// StartTranslationWorker binds the NATS queue consumer to Gemini and Discord Webhooks.
func StartTranslationWorker(ctx context.Context, natsMgr *queue.NatsManager, gemini *translator.GeminiService, multiplexer *discord.WebhookMultiplexer) {
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
		// For the sake of this setup, we assume the bot uses a static Webhook Token 
		// Or the multiplexer has logic to check cache. We use dummy placeholders here 
		// because Discord HTTP Interactions do not automatically provide webhook URLs unless we use the Followup API.
		
		// In a real Discord Interaction, you can "follow up" using the interaction token.
		// However, for strict Webhook Multiplexing as designed:
		err = multiplexer.ExecuteInline(ctx, job.ChannelID, "STATIC_TOKEN_HERE", job.OriginalUsername, job.AvatarURL, "🌍", translatedText)
		if err != nil {
			log.Printf("Multiplexer webhook execution failed: %v", err)
		}
	})

	if err != nil {
		log.Fatalf("Failed to start NATS consumer: %v", err)
	}
}
