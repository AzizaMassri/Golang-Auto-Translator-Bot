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
        "description": "e.g. English OR English, Spanish (comma separated)",
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
    "name": "reset_languages",
    "type": 1,
    "description": "Wipe translation databases for the entire server, or target a specific player",
    "default_member_permissions": "8",
    "options": [
      {
        "name": "target",
        "description": "Select a user to reset. Leave blank to wipe the entire server.",
        "type": 6,
        "required": false
      }
    ]
  },
  {
    "name": "force_language",
    "type": 1,
    "description": "Forcefully assign a persistent translation language to a specific user",
    "default_member_permissions": "8",
    "options": [
      {
        "name": "target",
        "description": "Select the user",
        "type": 6,
        "required": true
      },
      {
        "name": "language",
        "description": "The target language (e.g. Spanish, English)",
        "type": 3,
        "required": true
      }
    ]
  },
  {
    "name": "view_languages",
    "type": 1,
    "description": "View a list of all actively mapped player languages in the Database",
    "default_member_permissions": "8"
  },
  {
    "name": "help",
    "type": 1,
    "description": "Pull up the GlobalSync Guide and commands menu"
  }
]' \
  "https://discord.com/api/v10/applications/$APP_ID/commands"

echo -e "\n\nCommand registration dispatched!"
