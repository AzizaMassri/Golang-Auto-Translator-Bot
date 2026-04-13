#!/bin/bash
# deploy_vm.sh
# Safely stops the agent, pulls updates from Github, rebuilds, and restarts.

echo "Initiating globalsync VM deployment..."

# 1. Gracefully shut down current bot (handled by systemd sending SIGTERM)
echo "Stopping globalsync.service gracefully..."
sudo systemctl stop globalsync.service

# 2. Pull latest git updates
echo "Pulling latest Github changes..."
git pull origin main

# 3. Ensure dependencies are met and DBs are up
echo "Verifying docker infrastructure..."
docker-compose up -d

# 4. Rebuild native Go binary for maximum edge performance
echo "Compiling GlobalSync native binary..."
go build -o bin/globalsync ./cmd/globalsync

# 5. Restart the bot cleanly
echo "Starting globalsync.service..."
sudo systemctl start globalsync.service

echo "Checking status..."
sudo systemctl status globalsync.service --no-pager
