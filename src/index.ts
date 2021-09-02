import './dotenv_config';

import * as Database from 'better-sqlite3';
import * as TelegramBot from 'node-telegram-bot-api';
import {exit} from 'process';

console.log('Connecting the database');
const db = new Database('archive/database.sqlite', {verbose: console.log});

console.log('Creating the tables');
db.exec(`
CREATE TABLE IF NOT EXISTS "trip" (
  "id"	INTEGER PRIMARY KEY AUTOINCREMENT,
  "chat_id" TEXT NOT NULL,
  "name"	TEXT NOT NULL
);
`);

db.exec(`
CREATE TABLE IF NOT EXISTS "car" (
    "id"	INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "trip_id"	INTEGER NOT NULL,
    "user_id"	TEXT NOT NULL,
    "name"	TEXT NOT NULL,
    "passengers" INTEGER,
    "max_passengers" INTEGER
);
`);

db.exec(`
CREATE TABLE IF NOT EXISTS "passenger" (
    "id"	INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "car_id"	INTEGER NOT NULL,
    "user_id"	TEXT NOT NULL,
    "name"	TEXT NOT NULL
);
`);

console.log('Configuring the token');

if(!process.env.TOKEN)
{
    console.error('No token provided!');
    exit(1);
}

const token = process.env.TOKEN;

console.log(`Token ${token.substr(0, 3)}...${token.substr(token.length - 3)}`);

const bot = new TelegramBot(token, {polling: true});


bot.onText(/\/trip/, (msg) =>
{
    const chat_id = msg.chat.id;

    bot.sendMessage(chat_id, 'Please send the command as /trip [name of the trip]');
});

bot.onText(/\/trip (.+)/, (msg, match) =>
{
    const chat_id = msg.chat.id;

    if(!match)
        return;

    const trip_name: string = match[1].trim();

    const insert = db.prepare('INSERT INTO trip (chat_id, name) VALUES (@chat_id, @trip_name)');

    let result;

    try
    {
        result = insert.run({
            chat_id,
            trip_name,
        });
    }
    catch(error)
    {
        console.error(error);
        bot.sendMessage(chat_id, `Operation not completed for unexpected reason!`);
        return;
    }

    const opts = {
        reply_markup: {
            inline_keyboard: [
                [
                    {
                        text: 'Add 🚙',
                        callback_data: `add_car_${result.lastInsertRowid}`
                    }
                ]
            ]
        }
    };

    bot.sendMessage(chat_id, `📆 ${trip_name}`, opts);
});

function prepare_text_message(trip_id: number)
{
    const {name} = db.prepare("SELECT name FROM trip where trip.id = @trip_id").get({trip_id});

    const passengers = db.prepare("SELECT passenger.name as username, car.name as car_name FROM passenger JOIN car ON car.id = passenger.car_id where car.trip_id = @trip_id ORDER BY car.name").all({trip_id});

    let text = `📆 ${name}\n\n`

    const car_dictionary = {};

    passengers.map(passenger =>
    {
        let s = '';

        if(!(passenger.car_name in car_dictionary))
            car_dictionary[passenger.car_name] = [];

        car_dictionary[passenger.car_name].push(passenger.username);
    });

    for(const car in car_dictionary)
    {
        text += `🚙 ${car} [${car_dictionary[car].length}/5]:\n`;

        for(const username of car_dictionary[car])
            text += `- ${username}\n`;

        text += '\n'
    }

    return text;
}

function handle_add_car(callback_query: any, chat_id: number, msg: any, trip_id: number, user_id: number, username: string)
{
    const is_trip_existing = db.prepare("SELECT id FROM trip where chat_id = @chat_id AND id = @trip_id").get({chat_id, trip_id});

    if(!is_trip_existing)
    {
        bot.sendMessage(chat_id, `Operation not completed, no trip found.`);
        return;
    }

    const is_car_existing = db.prepare("SELECT id FROM car where trip_id = @trip_id AND user_id = @user_id").get({trip_id, user_id});

    if(is_car_existing)
    {
        bot.sendMessage(chat_id, `Operation not completed, car already added!`);
        return;
    }

    const insert = db.prepare('INSERT INTO car (trip_id, user_id, name) VALUES (@trip_id, @user_id, @name)');

    try
    {
        insert.run({
            trip_id,
            user_id,
            name: username
        });
    }
    catch(error)
    {
        console.error(error);
        bot.sendMessage(chat_id, `Operation not completed for unexpected reason!`);
        return;
    }

    const cars = db.prepare("SELECT id, name FROM car where trip_id = @trip_id").all({trip_id});

    const cars_button = cars.map(car => ([{
        text: `Join ${car.name}`,
        callback_data: `join_${car.id}`
    }]));

    const opts = {
        chat_id: msg.chat.id,
        message_id: msg.message_id,
        reply_markup: {
            inline_keyboard: [
                ...cars_button,
                [{
                    text: 'Add 🚙',
                    callback_data: `add_car_${trip_id}`
                }]
            ]
        }
    };

    const text = prepare_text_message(trip_id);
    bot.editMessageText(text, opts);
}

function handle_jump_in_car(callback_query: any, chat_id: number, msg: any, car_id: number, user_id: number, username: string)
{
    const {trip_id} = db.prepare("SELECT trip_id FROM car where car.id = @car_id").get({car_id});
    const is_passenger_existing = db.prepare("SELECT passenger.id, passenger.car_id FROM passenger JOIN car ON passenger.car_id = car.id where car.trip_id = @trip_id AND passenger.user_id = @user_id").get({trip_id, user_id});

    try
    {
        if(is_passenger_existing)
        {
            if(is_passenger_existing.car_id == car_id)
                return;

            const update = db.prepare('UPDATE passenger SET car_id = @car_id WHERE id = @id');;
            update.run({
                id: is_passenger_existing.id,
                car_id,
            });
        }
        else
        {
            const insert = db.prepare('INSERT INTO passenger (car_id, user_id, name) VALUES (@car_id, @user_id, @name)');
            insert.run({
                car_id,
                user_id,
                name: username
            });
        }
    }
    catch(error)
    {
        console.error(error);
        bot.sendMessage(chat_id, `Operation not completed for unexpected reason!`);
        return;
    }

    const text = prepare_text_message(trip_id);

    const cars = db.prepare("SELECT id, name FROM car where trip_id = @trip_id").all({trip_id});
    const cars_button = cars.map(car => ([{
        text: `Join ${car.name}`,
        callback_data: `join_${car.id}`
    }]));

    const opts = {
        chat_id: msg.chat.id,
        message_id: msg.message_id,
        reply_markup: {
            inline_keyboard: [
                ...cars_button,
                [{
                    text: 'Add 🚙',
                    callback_data: `add_car_${trip_id}`
                }]
            ]
        }
    };

    bot.editMessageText(text, opts);
}

bot.on('callback_query', (callback_query) =>
{
    if(!callback_query || !callback_query?.message?.chat.id || !callback_query.data)
        return;

    const chat_id = callback_query?.message?.chat.id;
    const action = callback_query.data;
    const from = callback_query.from;
    const msg = callback_query.message;

    console.log(action)
    try
    {
        let username = from?.username;

        if(!username)
        {
            if(from.first_name?.length > 0 && from?.last_name && from.last_name.length > 0)
                username = `${from?.first_name[0]}.${from?.last_name}`;
            else
                username = `ID:${from.id}`;
        }

        if(action.startsWith('add_car_'))
            handle_add_car(callback_query, chat_id, msg, parseInt(action.split('add_car_')[1], 10), from.id, username)

        if(action.startsWith('join_'))
            handle_jump_in_car(callback_query, chat_id, msg, parseInt(action.split('join_')[1], 10), from.id, username);
    }
    catch(error)
    {
        console.error(error);
    }
});

bot.onText(/\/start/, (msg) =>
{
    const chatId = msg.chat.id;
    bot.sendMessage(chatId, 'Hello I\'m car organizer bot!');
});
