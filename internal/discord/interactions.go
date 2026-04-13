package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"globalsync/internal/queue"
)

type InteractionHandler struct {
	NatsMgr *queue.NatsManager
}

func NewInteractionHandler(nm *queue.NatsManager) *InteractionHandler {
	return &InteractionHandler{NatsMgr: nm}
}

// HandleInteraction processes stateless HTTP webhooks sent by Discord.
func (ih *InteractionHandler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !verifyDiscordSignature(r, body) {
		log.Println("Unauthorized Discord Ping Blocked.")
		http.Error(w, "Invalid Request Signature", http.StatusUnauthorized)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Invalid JSON: %v", string(body))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Discord Ping Event
	if t, ok := payload["type"].(float64); ok && t == 1 {
		log.Println("Received PING from Discord gateway - Validated!")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"type": 1}) // PONG
		return
	}

	// Application Command Trigger (/translate or /help)
	if t, ok := payload["type"].(float64); ok && t == 2 {
		data := payload["data"].(map[string]interface{})
		commandName := data["name"].(string)

		if commandName == "help" {
			log.Println("Dispatching native /help menu.")
			
			// Instant Response with a Rich Embed
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, // CHANNEL_MESSAGE_WITH_SOURCE
				"data": map[string]interface{}{
					"embeds": []map[string]interface{}{
						{
							"title":       "🌍 GlobalSync Bot Guide",
							"description": "Welcome to GlobalSync! I use highly optimized Gemini AI pipelines to cleanly translate your messages instantly.\n\n**How to use:**\nType `/translate` and feed me the text and the target language. I will execute a Webhook that mimics your username and avatar, but translates the text directly into the chat for your international communities to read!\n\n**Enterprise Optimization:**\nIf you have purchased a premium Discord Subscription to GlobalSync, you are automatically routed to our massive `Gemini Pro` pipeline for deep cultural slang intelligence.",
							"color":       3447003, // Blue Color
						},
					},
					"flags": 64, // EPHEMERAL - Only the user who triggered it sees it
				},
			})
			return
		}

		if commandName == "translate" {
			log.Println("Received Application Command. Dispatching to NATS...")

			// Mock payload parsing (In prod, iterate over data["options"])
			job := queue.TranslationJob{
				ChannelID:        "mock-id",
				OriginalUsername: "EndUser",
				AvatarURL:        "",
				Content:          "Sample text to translate",
				TargetLanguage:   "Spanish",
				IsEnterprise:     false,
			}

			// Detect Discord Native Monetization / Subscriptions on the Event
			if entitlements, ok := payload["entitlements"].([]interface{}); ok && len(entitlements) > 0 {
				log.Println("Premium Subscription detected! Elevating to Gemini Pro.")
				job.IsEnterprise = true
			}

			ih.NatsMgr.PublishJob(job)

			// Defer response indicating we are "thinking..." (DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{"type": 5})
			return
		}
	}
}

// verifyDiscordSignature extracts headers and securely validates the payload
func verifyDiscordSignature(r *http.Request, body []byte) bool {
	pubKeyHex := os.Getenv("DISCORD_PUBLIC_KEY")
	if pubKeyHex == "" {
		log.Println("Error: DISCORD_PUBLIC_KEY environment variable is empty!")
		return false 
	}

	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		log.Printf("Error decoding public key hex: %v", err)
		return false
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	signature := r.Header.Get("X-Signature-Ed25519")
	timestamp := r.Header.Get("X-Signature-Timestamp")

	sigBytes, err := hex.DecodeString(signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		log.Println("Invalid signature format.")
		return false
	}

	msg := append([]byte(timestamp), body...)
	return ed25519.Verify(pubKey, msg, sigBytes)
}
