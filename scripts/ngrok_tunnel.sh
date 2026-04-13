#!/bin/bash
# ngrok_tunnel.sh
# Initiates a secure tunnel to expose the local Go environment to Discord

echo "======================================"
echo "    GlobalSync Ngrok Tunnel Setup   "
echo "======================================"

if ! command -v ngrok &> /dev/null; then
    echo "❌ Error: ngrok is not installed or not in PATH."
    echo "To install, visit: https://ngrok.com/download"
    echo "For Windows: choco install ngrok"
    echo "For Mac: brew install ngrok/ngrok/ngrok"
    echo "For Linux: apt install ngrok"
    exit 1
fi

echo "✅ ngrok detected! Launching tunnel on HTTP port 8080..."
echo "Wait for the dashboard to open, then copy the 'Forwarding' URL."
echo "Paste the following exactly into Discord Developer Portal 'Interactions Endpoint URL':"
echo "👉 https://<your-ngrok-id>.ngrok.app/interactions"
echo "======================================"

# Run ngrok for HTTP port 8080
ngrok http 8080
