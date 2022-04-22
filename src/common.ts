import * as Database from 'better-sqlite3';

import {db} from './database';

export function insert_trip(chat_id: string | number, trip_name: string): Database.RunResult
{
    const insert = db.prepare('INSERT INTO trip (chat_id, name) VALUES (@chat_id, @trip_name)');

    let result: Database.RunResult;

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
        throw new Error('Operation not completed for unexpected reason!')
    }

    return result;
}

export function insert_trip_message_id(trip_id: string, message_id: string | number)
{
    const update = db.prepare('UPDATE trip SET message_id = @message_id WHERE id = @id');
    update.run({
        id: trip_id,
        message_id,
    });
}

export function handle_add_car(chat_id: string | number, trip_id: number, user_id: string | number, username: string): {id: string, name: string}[]
{
    const is_trip_existing = db.prepare("SELECT id FROM trip where chat_id = @chat_id AND id = @trip_id").get({chat_id, trip_id});

    if(!is_trip_existing)
        throw new Error(`Operation not completed, no trip found.`);

    const is_car_existing = db.prepare("SELECT id FROM car where trip_id = @trip_id AND user_id = @user_id").get({trip_id, user_id});

    if(is_car_existing)
        throw new Error(`Operation not completed, car already added!`);

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
        throw new Error(`Operation not completed for unexpected reason!`);
    }

    const cars = db.prepare("SELECT id, name FROM car where trip_id = @trip_id").all({trip_id});

    return cars;
}

export function update_car_seats(max_passenger_number: number, chat_id: string | number, user_id: string | number): {cars: {id: string, name: string}[], trip_id: number}
{
    const is_car_existing = db.prepare("SELECT car.id, car.trip_id FROM car JOIN trip ON car.trip_id = trip.id where car.user_id = @user_id and trip.chat_id = @chat_id ORDER BY car.id DESC LIMIT 1").get({user_id, chat_id});

    if(!is_car_existing)
        throw new Error(`Operation not completed, no car found.`);

    const trip_id = is_car_existing.trip_id;
    try
    {
        const update = db.prepare('UPDATE car SET max_passengers = @max_passenger_number WHERE id = @id');
        update.run({
            id: is_car_existing.id,
            max_passenger_number,
        });
    }
    catch(error)
    {
        console.error(error);
        throw new Error(`Operation not completed for unexpected reason!`);
    }

    const cars = db.prepare("SELECT id, name FROM car where trip_id = @trip_id").all({trip_id});

    return {cars, trip_id};
}

export function handle_jump_in_car(car_id: number, user_id: string | number, username: string): {id: string, name: string}[]
{
    const {trip_id} = db.prepare("SELECT trip_id FROM car where car.id = @car_id").get({car_id});
    const is_passenger_existing = db.prepare("SELECT passenger.id, passenger.car_id FROM passenger JOIN car ON passenger.car_id = car.id where car.trip_id = @trip_id AND passenger.user_id = @user_id").get({trip_id, user_id});

    if(is_passenger_existing)
    {
        if(is_passenger_existing.car_id == car_id)
            throw new Error(`Operation not completed, you are already in that car!`);

        try
        {
            const update = db.prepare('UPDATE passenger SET car_id = @car_id WHERE id = @id');
            update.run({
                id: is_passenger_existing.id,
                car_id,
            });
        }
        catch(error)
        {
            console.error(error);
            throw new Error(`Operation not completed for unexpected reason!`);
        }
    }
    else
    {
        try
        {
            const insert = db.prepare('INSERT INTO passenger (car_id, user_id, name) VALUES (@car_id, @user_id, @name)');
            insert.run({
                car_id,
                user_id,
                name: username
            });
        }
        catch(error)
        {
            console.error(error);
            throw new Error(`Operation not completed for unexpected reason!`);
        }
    }

    const cars = db.prepare("SELECT id, name FROM car where trip_id = @trip_id").all({trip_id});

    return cars;
}

export function prepare_text_message(trip_id: number): string
{
    const {name} = db.prepare("SELECT name FROM trip where trip.id = @trip_id").get({trip_id});

    const passengers = db.prepare("SELECT passenger.name as username, car.name as car_name, car.max_passengers as max_passengers FROM passenger JOIN car ON car.id = passenger.car_id where car.trip_id = @trip_id ORDER BY car.name").all({trip_id});

    let text = `📆 *${name}*\n\n`

    const car_dictionary = {};

    passengers.map(passenger =>
    {
        if(!(passenger.car_name in car_dictionary))
        {
            car_dictionary[passenger.car_name] = {
                info: {
                    max_passengers: passenger.max_passengers
                },
                passenger: []
            };
        }

        car_dictionary[passenger.car_name].passenger.push(passenger.username);
    });

    for(const car in car_dictionary)
    {
        const is_car_full = car_dictionary[car].passenger.length >= (car_dictionary[car].info.max_passengers || 5) ? true : false;

        text += `${is_car_full ? '🚗' : '🚙'} *${car}* [${car_dictionary[car].passenger.length}/${car_dictionary[car].info.max_passengers || 5}] ${is_car_full ? '🚫' : ''}:\n`;

        for(const username of car_dictionary[car].passenger)
            text += `• ${username}\n`;

        text += '\n'
    }

    return text;
}
