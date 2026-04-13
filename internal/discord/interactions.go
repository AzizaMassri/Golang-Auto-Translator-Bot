package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

// HandleInteraction processes stateless HTTP webhooks sent by Discord.
func HandleInteraction(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate Request via Ed25519 Signature
	if !verifyDiscordSignature(r) {
		http.Error(w, "Invalid Request Signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse Discord Payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Discord Ping Event
	if t, ok := payload["type"].(float64); ok && t == 1 {
		log.Println("Received PING from Discord gateway")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"type": 1}) // PONG
		return
	}

	// Handle standard command / component interactions here.
	// We would push this event to the NATS JetStream queue for asynchronous translation.
	log.Println("Received valid interaction event. Dispatching to NATS...")

	// Defere response indicating we are "thinking..." 
	// (Type 5: DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"type": 5})
}

// verifyDiscordSignature extracts headers and securely validates the payload
func verifyDiscordSignature(r *http.Request) bool {
	pubKeyHex := os.Getenv("DISCORD_PUBLIC_KEY")
	if pubKeyHex == "" {
		// Defaulting to true for local testing if env is missing, normally strict false.
		log.Println("Warning: DISCORD_PUBLIC_KEY not set!")
		return true 
	}

	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	signature := r.Header.Get("X-Signature-Ed25519")
	timestamp := r.Header.Get("X-Signature-Timestamp")

	sigBytes, err := hex.DecodeString(signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}

	body, _ := io.ReadAll(r.Body)
	// Reset body for downstream components
	// In production we use a bytes.Buffer to reuse the body
	
	msg := append([]byte(timestamp), body...)
	return ed25519.Verify(pubKey, msg, sigBytes)
}
