package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"globalsync/internal/discord"
)

// loadEnv parses the .env file recursively without external packages
func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Println("No .env file found. Relying on system environment variables.")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(parts[1], `"'`)
				os.Setenv(parts[0], val)
			}
		}
	}
}

func main() {
	loadEnv()

	log.Println("Booting GlobalSync (Go + NATS + Dragonfly) Architecture...")

	http.HandleFunc("/interactions", discord.HandleInteraction)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
	}

	// Run server in a goroutine so it doesn't block
	go func() {
		log.Printf("Listening for Discord HTTP Webhooks on %s/interactions", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FAILED to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Gracefully shutting down GlobalSync server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting gracefully")
}
