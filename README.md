# GlobalSync Bot

GlobalSync Bot is an ultra-fast, stateless Discord translation bot designed exclusively for high-traffic environments. 

Rather than relying on Discord WebSockets—which scales poorly and hogs memory—GlobalSync uses **Stateless HTTP Interactions**, dropping typical server footprint significantly (~20MB RAM) while executing translation requests directly against Google's Gemini models in real time.

## 🚀 Architecture Highlights

- **Language:** Written natively in `Go (Golang)` for maximum concurrency.
- **Message Broker:** Uses **NATS JetStream** instead of RabbitMQ for lightweight, low-latency queues to absorb global api rate limits perfectly.
- **Caching / Webhook Pooling:** Uses **DragonflyDB** (a multi-threaded Redis replacement doing ~4M ops/sec) to pool webhook IDs and dynamically recycle them to stay underneath Discord's 15-webhook-per-channel ceilings.
- **Model Layer:** Integrates the Agent Development Kit powered by `Gemini 2.5 Flash` (Standard) & `Gemini 3.1 Pro` (Enterprise/Accuracy).
- **gRPC Ready:** Bootstrapped via Protobufs to natively extract workers onto separate VM racks dynamically.

---

## 🛠️ Setup Guide

### 1. Requirements
Ensure your host machine has installed:
- [Docker](https://www.docker.com/) & Docker-Compose (For Dragonfly, NATS, and Prometheus)
- [Go 1.21+](https://go.dev/)

### 2. Environment Variables
To securely bind your API integrations, create a standard `.env` file or export these onto your machine:

```env
DISCORD_PUBLIC_KEY="your-discord-app-public-key"
GEMINI_API_KEY="your-google-ai-studio-key"
PORT="8080" # Optional, defaults to 8080
```
> **Note:** The `DISCORD_PUBLIC_KEY` controls the ED25519 cryptography handshake ensuring malicious bots cannot spoof commands onto your webhook endpoint.

### 3. Running the Infrastructure

Launch the caching layer and message broker utilizing docker:
```bash
docker-compose up -d
```
This boots:
* **DragonflyDB** on `:6379`
* **NATS Jetstream** on `:4222` and `:8222`
* **Prometheus Telemetry** on `:9090`

### 4. Running the Bot
Once the infrastructure containers are healthy, boot the Go listener:

```bash
go run cmd/globalsync/main.go
```

The system will now securely await incoming HTTP Discord Event Pings via `localhost:8080/interactions`.

*(If testing locally before deploying to an actual server, launch a reverse proxy like `ngrok http 8080` to safely bounce Discord requests down to your localhost).*
