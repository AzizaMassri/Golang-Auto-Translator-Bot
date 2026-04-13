#!/bin/bash
# register_commands.sh
# Sends an HTTP POST to Discord to globally register your slash commands
# Without this, Discord won't know to show the /translate command UI to users!

APP_ID=$1
BOT_TOKEN=$2

if [ -z "$APP_ID" ] || [ -z "$BOT_TOKEN" ]; then
    echo "Usage: ./register_commands.sh <APPLICATION_ID> <BOT_TOKEN>"
    exit 1
fi

echo "Registering /translate command to Discord Global Registry..."

curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bot $BOT_TOKEN" \
  -d '{
    "name": "translate",
    "type": 1,
    "description": "Translates your text instantly",
    "options": [
      {
        "name": "text",
        "description": "The text to translate",
        "type": 3,
        "required": true
      },
      {
        "name": "language",
        "description": "Target Language (e.g. Spanish, French)",
        "type": 3,
        "required": true
      }
    ]
  }' \
  "https://discord.com/api/v10/applications/$APP_ID/commands"

echo -e "\n\nCommand registration dispatched!"
