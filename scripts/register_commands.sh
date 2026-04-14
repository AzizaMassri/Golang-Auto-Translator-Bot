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

curl -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bot $BOT_TOKEN" \
  -d '[
  {
    "name": "language",
    "type": 1,
    "description": "Set your native language for Auto-Translation",
    "options": [
      {
        "name": "selection",
        "description": "What language do you speak? (e.g. Spanish, Japanese, English)",
        "type": 3,
        "required": true
      }
    ]
  },
  {
    "name": "toggle",
    "type": 1,
    "description": "Toggle Auto-Translation ON or OFF for your messages",
    "options": [
      {
        "name": "state",
        "description": "Select ON or OFF",
        "type": 3,
        "required": true,
        "choices": [
            { "name": "ON", "value": "ON" },
            { "name": "OFF", "value": "OFF" }
        ]
      }
    ]
  },
  {
    "name": "help",
    "type": 1,
    "description": "Pull up the GlobalSync Guide and commands menu"
  }
]' \
  "https://discord.com/api/v10/applications/$APP_ID/commands"

echo -e "\n\nCommand registration dispatched!"
