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
	"globalsync/internal/queue"
	"globalsync/internal/translator"
	"globalsync/internal/worker"

	"github.com/redis/go-redis/v9"
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

	// 1. Initialize Dragonfly DB
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	multiplexer := discord.NewWebhookMultiplexer(rdb)

	// 2. Initialize Gemini AI
	geminiCtx := context.Background()
	gemini, err := translator.NewGeminiService(geminiCtx, os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatalf("Failed to bind Gemini Client: %v", err)
	}

	// 3. Initialize NATS JetStream
	natsMgr := &queue.NatsManager{}
	// Use docker networking or localhost accordingly
	if err := natsMgr.Connect("nats://localhost:4222"); err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}

	// 4. Start internal worker pipeline
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	worker.StartTranslationWorker(workerCtx, natsMgr, gemini, multiplexer)

	// 5. Inject NATS into the Discord handler
	handler := discord.NewInteractionHandler(natsMgr)
	http.HandleFunc("/interactions", handler.HandleInteraction)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
	}

	go func() {
		log.Printf("Listening for Discord HTTP Webhooks on %s/interactions", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FAILED to start HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Gracefully shutting down GlobalSync server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	natsMgr.NC.Close()
	log.Println("Server exiting gracefully")
}
