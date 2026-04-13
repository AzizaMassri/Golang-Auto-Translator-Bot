package main

import (
	"log"
	"net/http"
	"os"

	"globalsync/internal/discord"
)

func main() {
	log.Println("Booting GlobalSync (Go + NATS + Dragonfly) Architecture...")

	// Initialize stateless Discord HTTP Interaction Handler
	http.HandleFunc("/interactions", discord.HandleInteraction)

	// Pull port from env or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening for Discord HTTP Webhooks on :%s/interactions", port)
	
	// Start HTTP Server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("FAILED to start HTTP server: %v", err)
	}
}
