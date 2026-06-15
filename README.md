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

With the Token of the bot.

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

## Preview
![car_trip_output](https://user-images.githubusercontent.com/6942680/131878039-33278302-6d89-408c-aeb1-f0034672b234.gif)


## Usage

`/trip name of the trip`

Click on the button 'Add'.

Select the car you want to join.
