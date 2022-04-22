import {AttachmentAction, LogLevel, WebClient} from '@slack/web-api';
import * as bodyParser from 'body-parser';
import * as cors from 'cors';
import * as express from 'express';
import {exit} from 'process';

import {
    handle_add_car,
    handle_jump_in_car,
    insert_trip,
    insert_trip_message_id,
    prepare_text_message,
    update_car_seats,
} from './common';
import {db} from './database';

if(!process.env.SLACK_TOKEN)
{
    console.error('No slack token provided!');
    exit(1);
}

const client = new WebClient(process.env.SLACK_TOKEN, {
    logLevel: LogLevel.DEBUG
});

const app = express();
app.use(bodyParser.urlencoded({
    extended: true
}));
app.use(bodyParser.json());
app.use(cors());
const port = process.env.SLACK_HTTP_PORT || 3000

app.get('/', async (_req: TypedRequestBody<any>, res: TypedResponseSend<any>) =>
{
    res.send('Nothing to do here');
});

app.post('/', async (req: TypedRequestBody<any>, res: TypedResponseSend<any>) =>
{
    try
    {
        let command = req.body.command;

        switch(command)
        {
            case '/trip':
            case '/trip_test':
            return await create_new_trip(req, res);

            case '/seats':
            return await update_car_seats_wrapper(req, res);

            default:
                return res.send('Command not found');
        }
    }
    catch(error)
    {
        console.error(error);
        await client.chat.postEphemeral({
            channel: req.body.channel_id,
            user: req.body.user_id,
            text: `Unable to complete the operation due to ${error}`,
        });
    }

    res.send();
});

app.post('/webhook', async (req: TypedRequestBody<any>, res: TypedResponseSend<any>) =>
{
    const payload = JSON.parse(req.body.payload);

    try
    {
        if(payload.actions[0].value.startsWith('add_car'))
            return await add_car(req, res, payload);

        if(payload.actions[0].value.startsWith('join_'))
            return await join_car(req, res, payload);

        return res.send('Command not found');
    }
    catch(error)
    {
        console.error(error);
        await client.chat.postEphemeral({
            channel: payload.channel.id,
            user: payload.user.id,
            text: `Unable to complete the operation due to ${error}`,
        });
    }

    res.send();
});

app.listen(port, () =>
{
    console.log(`App listening on port ${port}`)
})

export interface TypedRequestBody<T> extends Express.Request {
    body: T
}

export interface TypedResponseSend<T> extends Express.Response {
    send: T
}

async function create_new_trip(req: TypedRequestBody<{channel_id: string, text: string}>, res: TypedResponseSend<any>)
{
    const channel_id = req.body.channel_id;
    const trip_name = req.body.text.trim();

    const db_result = insert_trip(channel_id, trip_name);
    const trip_id = db_result.lastInsertRowid.toString();

    res.send();
    const result = await client.chat.postMessage({
        channel: channel_id,
        text: `📆 *${trip_name}*`,
        response_type: 'in_channel',
        attachments: [{
                text: 'Add a new 🚙 or jump in',
                fallback: 'You are unable to add a car',
                callback_id: `add_car_${trip_id}`,
                color: '#3AA3E3',
                actions: [
                    {
                        name: 'add car',
                        text: 'Add 🚙',
                        type: 'button',
                        value: `add_car_${trip_id}`
                    }
                ]
            }
        ]
    });

    insert_trip_message_id(trip_id, result.ts!);
}

interface AddCar
{
    type: string,
    actions: any[],
    callback_id: string;
    team:{
        id: string,
        domain: string;
    },
    channel:{
        id: string,
        name: string,
    },
    user:{
        id: string,
        name: string,
    },
    // 'action_ts':'xxxxxxxxxx.xxxxxx',
    message_ts: string,
    // 'attachment_id':'1',
    // 'token':'xxxxxxxxxxxxxxxxxxxxxxxxxx',
    // 'is_app_unfurl':false,
    // 'enterprise':null,
    // 'is_enterprise_install':false,
    response_url: string,
    trigger_id: string

}

async function add_car(_req: TypedRequestBody<any>, res: TypedResponseSend<any>, data: AddCar)
{
    const trip_id = parseInt(data.actions[0].value.split('add_car_')[1], 10);
    const cars = handle_add_car(data.channel.id, trip_id, data.user.id, data.user.name);

    const cars_button: AttachmentAction[] = cars.map(car => ({
        name: `Join ${car.name}`,
        text: `Join ${car.name}`,
        type: 'button',
        value: `join_${car.id}`
    }));


    const text = prepare_text_message(trip_id);
    const old_message_reference = db.prepare("SELECT chat_id, message_id FROM trip where trip.id = @trip_id").get({trip_id});

    res.send();
    await client.chat.update({
        channel: data.channel.id,
        ts: old_message_reference.message_id,
        text,
        response_type: 'in_channel',
        replace_original: true,
        attachments: [{
                text: 'Add a new 🚙 or jump in',
                fallback: 'You are unable to add a car',
                callback_id: `add_car_${trip_id}`,
                color: '#3AA3E3',
                actions: [
                    ...cars_button,
                    {
                        name: 'add car',
                        text: 'Add 🚙',
                        type: 'button',
                        value: `add_car_${trip_id}`
                    },
                ]
            }
        ]
    });
}

async function join_car(_req: TypedRequestBody<any>, res: TypedResponseSend<any>, data: AddCar)
{
    const car_id = parseInt(data.actions[0].value.split('join_')[1], 10);
    const cars = handle_jump_in_car(car_id, data.user.id, data.user.name);

    const {trip_id} = db.prepare("SELECT trip_id FROM car where car.id = @car_id").get({car_id});

    const cars_button: AttachmentAction[] = cars.map(car => ({
        name: `Join ${car.name}`,
        text: `Join ${car.name}`,
        type: 'button',
        value: `join_${car.id}`
    }));

    const text = prepare_text_message(trip_id);
    const old_message_reference = db.prepare("SELECT chat_id, message_id FROM trip where trip.id = @trip_id").get({trip_id});

    res.send();
    await client.chat.update({
        channel: data.channel.id,
        ts: old_message_reference.message_id,
        text,
        response_type: 'in_channel',
        replace_original: true,
        attachments: [{
                text: 'Add a new 🚙 or jump in',
                fallback: 'You are unable to add a car',
                callback_id: `add_car_${trip_id}`,
                color: '#3AA3E3',
                actions: [
                    ...cars_button,
                    {
                        name: 'add car',
                        text: 'Add 🚙',
                        type: 'button',
                        value: `add_car_${trip_id}`
                    },
                ]
            }
        ]
    });
}

async function update_car_seats_wrapper(req: TypedRequestBody<{channel_id: string, text: string, response_url: string, user_id: string}>, res: TypedResponseSend<any>)
{
    const channel_id = req.body.channel_id;
    const max_passenger_number = parseInt(req.body.text.trim(), 10);
    const user_id = req.body.user_id;

    const {cars, trip_id} = update_car_seats(max_passenger_number, channel_id, user_id);

    const old_message_reference = db.prepare("SELECT chat_id, message_id FROM trip where trip.id = @trip_id").get({trip_id});

    const text = prepare_text_message(trip_id);

    const cars_button: AttachmentAction[] = cars.map(car => ({
        name: `Join ${car.name}`,
        text: `Join ${car.name}`,
        type: 'button',
        value: `join_${car.id}`
    }));

    await client.chat.update({
        channel: old_message_reference.chat_id,
        ts: old_message_reference.message_id,
        text,
        response_type: 'in_channel',
        replace_original: true,
        attachments: [{
                text: 'Add a new 🚙 or jump in',
                fallback: 'You are unable to add a car',
                callback_id: `add_car_${trip_id}`,
                color: '#3AA3E3',
                actions: [
                    ...cars_button,
                    {
                        name: 'add car',
                        text: 'Add 🚙',
                        type: 'button',
                        value: `add_car_${trip_id}`
                    },
                ]
            }
        ]
    });

    res.send({
        text: `Update your car info!`,
        response_type: 'ephemeral',
    });
}
