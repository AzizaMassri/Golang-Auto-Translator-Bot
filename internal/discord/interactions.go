package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

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
				"type": 4, 
				"data": map[string]interface{}{
					"content": "✅ Perfect! Your chat messages will now dynamically translate to match the languages of other active gamers in the channel! (And when they speak, you'll see translations in **" + langSelection + "**!)",
					"flags": 64, 
				},
			})
			return
		}
		if commandName == "toggle" {
			// Extract User ID
			var userID string
			if member, ok := payload["member"].(map[string]interface{}); ok {
				user := member["user"].(map[string]interface{})
				userID = user["id"].(string)
			} else {
				user := payload["user"].(map[string]interface{})
				userID = user["id"].(string)
			}

			options := data["options"].([]interface{})
			state := options[0].(map[string]interface{})["value"].(string)

			message := ""
			if state == "OFF" {
				ih.RDB.Set(r.Context(), "user:"+userID+":optout", "true", 0)
				message = "🛑 **Opt-Out Enabled**: The GlobalSync Gateway will now actively ignore your chat messages. They will no longer be intercepted, deleted, or translated."
				log.Printf("User %s opted OUT of translation routing.", userID)
			} else {
				ih.RDB.Set(r.Context(), "user:"+userID+":optout", "false", 0)
				message = "✅ **Opt-Out Disabled**: Auto-translation has been fully restored for your messages!"
				log.Printf("User %s opted BACK IN to translation routing.", userID)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, 
				"data": map[string]interface{}{
					"content": message,
					"flags": 64, 
				},
			})
			return
		}

		if commandName == "force_language" {
			options := data["options"].([]interface{})
			var targetUserID, langSelection string
			for _, opt := range options {
				o := opt.(map[string]interface{})
				if o["name"] == "target" {
					targetUserID = o["value"].(string)
				}
				if o["name"] == "language" {
					langSelection = o["value"].(string)
				}
			}

			ih.RDB.Set(r.Context(), "user:"+targetUserID+":language", langSelection, 0)
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, 
				"data": map[string]interface{}{
					"content": "🛡️ **Admin Override**: Successfully pinned targeted user translation routing to **" + langSelection + "**.",
					"flags": 64, 
				},
			})
			return
		}

		if commandName == "reset_languages" {
			var targetUserID string
			if opts, ok := data["options"].([]interface{}); ok && len(opts) > 0 {
				o := opts[0].(map[string]interface{})
				if o["name"] == "target" {
					targetUserID = o["value"].(string)
				}
			}

			ctx := r.Context()
			message := ""
			
			if targetUserID != "" {
				ih.RDB.Del(ctx, "user:"+targetUserID+":language")
				ih.RDB.Del(ctx, "user:"+targetUserID+":optout")
				message = "🛡️ **Admin Override**: Wiped translation tracking memory strictly for the targeted user."
			} else {
				keys1, _ := ih.RDB.Keys(ctx, "user:*:language").Result()
				keys2, _ := ih.RDB.Keys(ctx, "user:*:optout").Result()
				keys1 = append(keys1, keys2...)

				if len(keys1) > 0 {
					ih.RDB.Del(ctx, keys1...)
				}
				message = "💥 **Memory Flushed**: All user translation languages and opt-out flags have been completely wiped!"
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, 
				"data": map[string]interface{}{
					"content": message,
					"flags": 64, 
				},
			})
			return
		}

		if commandName == "view_languages" {
			ctx := r.Context()
			keys, _ := ih.RDB.Keys(ctx, "user:*:language").Result()
			
			if len(keys) == 0 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"type": 4, 
					"data": map[string]interface{}{
						"content": "📭 No user languages are currently assigned in the DB.",
						"flags": 64, 
					},
				})
				return
			}

			output := "📊 **GlobalSync Registry:**\n\n"
			for _, key := range keys {
				lang, _ := ih.RDB.Get(ctx, key).Result()
				parts := strings.Split(key, ":")
				if len(parts) == 3 {
					userID := parts[1]
					output += fmt.Sprintf("• <@%s> : **%s**\n", userID, lang)
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": 4, 
				"data": map[string]interface{}{
					"content": output,
					"flags": 64, 
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
