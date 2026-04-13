#!/bin/bash
# run_local.sh
# Quickly starts your local development environment

echo "Starting GlobalSync Local Development Environment..."

# 1. Start DragonflyDB, NATS, and Prometheus
docker-compose up -d

# 2. Wait a few seconds for dependencies to boot
echo "Waiting for infrastructure containers..."
sleep 3

# 3. Run the Go Bot natively
echo "Booting Go Bot..."
go run cmd/globalsync/main.go
