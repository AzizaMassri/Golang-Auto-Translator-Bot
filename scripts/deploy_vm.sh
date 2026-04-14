#!/bin/bash
# deploy_vm.sh
# Safely stops the agent, pulls updates from Github, rebuilds, and restarts.

set -e

# Ensure Go is in PATH
export PATH=$PATH:/usr/local/go/bin

echo "Initiating globalsync VM deployment..."

# 1. Change to repository root and shut down current bot
cd "$(dirname "$0")/.."
echo "Stopping globalsync.service gracefully..."
sudo systemctl stop globalsync.service || echo "Service not running or not loaded, continuing..."

# 2. Pull latest git updates
echo "Pulling latest GitHub changes..."
git pull origin main

# 3. Ensure dependencies are met and DBs are up
echo "Verifying Docker infrastructure..."
sudo docker compose -f docker-compose.yml up -d

# 4. Rebuild native Go binary for maximum edge performance
echo "Compiling GlobalSync native binary..."
mkdir -p bin
go build -o bin/globalsync ./cmd/globalsync

# 5. Restart the bot cleanly
echo "Restarting globalsync.service..."
sudo systemctl restart globalsync.service || (echo "Service not loaded, linking and starting..." && sudo ln -sf /opt/GlobalSync-Bot/scripts/globalsync.service /etc/systemd/system/globalsync.service && sudo systemctl daemon-reload && sudo systemctl enable globalsync.service && sudo systemctl start globalsync.service)

echo "Checking status..."
sudo systemctl status globalsync.service --no-pager
