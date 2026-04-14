package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"globalsync/internal/queue"
)

type InteractionHandler struct {
	NatsMgr *queue.NatsManager
	RDB     *redis.Client
}

func NewInteractionHandler(nm *queue.NatsManager, rdb *redis.Client) *InteractionHandler {
	return &InteractionHandler{NatsMgr: nm, RDB: rdb}
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

	// Application Command Trigger (/language or /help)
	if t, ok := payload["type"].(float64); ok && t == 2 {
		data := payload["data"].(map[string]interface{})
		commandName := data["name"].(string)

		if commandName == "help" {
			log.Println("Dispatching native /help menu.")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, // Instant ChannelMessageWithSource
				"data": map[string]interface{}{
					"embeds": []map[string]interface{}{
						{
							"title":       "🌍 GlobalSync Auto-Translation Architecture",
							"description": "Welcome to GlobalSync! I translate all messages natively in your channels.\n\n**How to use:**\nType `/language` and set the language you naturally speak. Whenever someone else sends a message in the channel, it will instantly translate it strictly into your chosen language, perfectly mimicking them!\n\n**Enterprise Optimization:**\nPremium channels instantly use `Gemini Pro` for zero latency cultural translation mapping.",
							"color":       3447003,
						},
					},
					"flags": 64, // Ephemeral
				},
			})
			return
		}

		if commandName == "language" {
			// Extract User ID determining if invoked in Server or DM
			var userID string
			if member, ok := payload["member"].(map[string]interface{}); ok {
				user := member["user"].(map[string]interface{})
				userID = user["id"].(string)
			} else {
				user := payload["user"].(map[string]interface{})
				userID = user["id"].(string)
			}

			// Extract User's selected language
			options := data["options"].([]interface{})
			langSelection := options[0].(map[string]interface{})["value"].(string)

			// Fast cache into Dragonfly
			ih.RDB.Set(r.Context(), "user:"+userID+":language", langSelection, 0)
			
			log.Printf("User %s pinned native language as: %s", userID, langSelection)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, // 4 = Instant response (Bypasses the 'thinking' bug completely)
				"data": map[string]interface{}{
					"content": "✅ Perfect! All international messages in your channels will now invisibly Auto-Translate into **" + langSelection + "** for you! (Other users will see it in their chosen language)",
					"flags": 64, // Ephemeral
				},
			})
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
