# Car Organizer Bot
## The easiest way to organize cars for a trip

This bot offer an easy way to organize a trip with friends or colleagues.
Create a new trip and add some available cars and ready to go!


## Telegram Bot

This code is the backend of a bot named [@car_organizer_bot](http://telegram.me/car_organizer_bot).
In order to run the bot locally you must create a file named `.env`

With the Token of the bot.

```
TOKEN=xxxxxxxxxx:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
NTBA_FIX_319=1
```

### Start node
To start the bot just write

```
mkdir archive
npm i
npm run start
```

### Docker container
To start the docker container

```
docker build -t car_organizer .
docker run -v ~/archive:/archive -d car_organizer:latest
```

## Preview
![car_trip_output](https://user-images.githubusercontent.com/6942680/131878039-33278302-6d89-408c-aeb1-f0034672b234.gif)


## Usage

`/trip name of the trip`

Click on the buttont 'Add`.

Select the car you want to join.
