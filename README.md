# Car Organizer Bot

[![ko-fi](https://img.shields.io/badge/Support%20me%20on%20Ko--fi-F16061?style=flat-square&logo=ko-fi&logoColor=white)](https://ko-fi.com/dangerblack)

A Telegram (and optionally Slack) bot for organizing car trips with friends.

Create a trip, add your car, and let people jump in.

## Features

- **Trip Management** — `/trip [name]` creates a new trip with interactive buttons
- **Add Car** — Click "Add 🚙" to offer your car
- **Join Car** — Click a friend's car to join it (moves you if already in another car)
- **Leave Trip** — Click "Leave trip" to remove yourself or your car
- **Seats** — `/seats [number]` to customize your car's capacity
- **Display Name** — `/name [name]` to set how you appear in trips
- **Slack support** — optional, via slash commands and Block Kit buttons

## Telegram Bot

Requires a Telegram bot token from [@BotFather](https://t.me/BotFather). Add the bot to a group and create a trip.

### Commands

| Command | Description |
|---|---|
| `/trip [name]` | Create a new trip |
| `/seats [n]` | Set the number of seats in your car |
| `/name [name]` | Set your display name |
| `/start` | Show help |
| `/help` | Show help |

All other actions (add car, join, leave) are done via inline buttons.

## Slack Bot (optional)

The bot also supports Slack via slash commands and Block Kit buttons. Omit `SLACK_TOKEN` to disable Slack entirely.

### Slack App Setup

1. Create a Slack app at https://api.slack.com/apps
2. Add **Bot Token Scopes**: `chat:write`, `chat:write.public`, `commands`
3. Install the app to your workspace
4. Set up **Slash Commands**:
   - `/trip` — `https://your-host/`
   - `/seats` — `https://your-host/`
   - `/name` — `https://your-host/`
5. Set **Interactivity & Shortcuts** → **Request URL**: `https://your-host/webhook`

### Exposing your local server

```
ngrok http 3000
```

Use the ngrok URL in the Slack app settings.

## Getting Started

### Prerequisites

- Go 1.24+
- Telegram bot token (required — set `TOKEN` in `.env`)

### Setup

```bash
# Clone and enter the directory
git clone <repo>
cd car_organizer_bot

# Create config
cp .env.example .env
# Edit .env and add your Telegram token

# Create data directory
mkdir archive

# Run
go run src/main.go
```

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `TOKEN` | ✅ | — | Telegram bot token |
| `DB_PATH` | | `./archive` | SQLite database storage path |
| `SLACK_TOKEN` | | — | Slack bot token (omit to disable Slack) |
| `SLACK_SIGNING_SECRET` | | — | Slack signing secret (omit to skip verification) |
| `SLACK_HTTP_PORT` | | `3000` | Port for the Slack HTTP server |

### Preview

![car_trip_output](https://user-images.githubusercontent.com/6942680/131878039-33278302-6d89-408c-aeb1-f0034672b234.gif)

## Docker Compose

```bash
docker compose up -d
```

### Docker

A pre-built image is available on Docker Hub (automatically updated on push to `main`):

```bash
docker run --env-file .env -v ./archive:/archive dangerblack/car-organizer-bot
```

### Build from source

```bash
docker build -t car_organizer .
docker run --env-file .env -v ./archive:/archive car_organizer
```
