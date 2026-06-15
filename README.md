# Car Organizer Bot
## The easiest way to organize cars for a trip

This bot offers an easy way to organize a trip with friends or colleagues.
Create a new trip and add some available cars and ready to go!


## Features

- **Trip Management**: Create trips with `/trip [name]`
- **Add Car**: Click "Add 🚙" to offer your car
- **Join Car**: Click "Join [name]" to jump into a friend's car
- **Seats**: Customize your car seats with `/seats [number]`
- **Remove Car**: Remove your car with `/remove`
- **Custom Name**: Set your display name with `/name [name]`

## Telegram Bot

This code is the backend of a bot named [@car_organizer_bot](http://telegram.me/car_organizer_bot).
In order to run the bot locally you must create a file named `.env`

```
TOKEN=xxxxxxxxxx:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
DB_PATH=./archive
```

### Run locally

```
mkdir archive
go run src/main.go
```

### Docker Compose

```yaml
services:
  car_organizer:
    build: .
    container_name: car_organizer
    environment:
      - TOKEN=xxxxxxxxxx:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
      - DB_PATH=/archive
    volumes:
      - ./archive:/archive
    restart: unless-stopped
```

### Docker

```
docker build -t car_organizer .
docker run --env-file .env -v ./archive:/archive car_organizer
```

## Slack Bot

The bot also supports Slack via slash commands and interactive buttons. Add these env vars to your `.env`:

```
SLACK_TOKEN=xoxb-your-bot-token
SLACK_SIGNING_SECRET=your-signing-secret
SLACK_HTTP_PORT=3000
```

### Slack App Setup

1. Create a Slack app at https://api.slack.com/apps
2. Add these **Bot Token Scopes**: `chat:write`, `chat:write.public`, `commands`
3. Install the app to your workspace
4. Set up **Slash Commands**:
   - `/trip` — Create a new trip: `https://your-host/`
   - `/seats` — Set car seats: `https://your-host/`
5. Set up **Interactive Components** → **Request URL**: `https://your-host/webhook`

### Exposing your local server

Use ngrok or a tunnel to expose your local server to Slack:

```
ngrok http 3000
```

Then use the ngrok URL as the Request URL in Slack app settings.

### Requirements

- Go 1.24+
- Telegram bot token (required — the bot always starts Telegram; set `TOKEN` in `.env`)

## Preview
![car_trip_output](https://user-images.githubusercontent.com/6942680/131878039-33278302-6d89-408c-aeb1-f0034672b234.gif)


## Usage

`/trip name of the trip`

Click on the button 'Add'.

Select the car you want to join.
