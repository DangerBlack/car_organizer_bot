import * as Database from 'better-sqlite3';

export const db = new Database('archive/database.sqlite', {verbose: console.log});

console.log('Creating the tables');
db.exec(`
CREATE TABLE IF NOT EXISTS "trip" (
  "id"	INTEGER PRIMARY KEY AUTOINCREMENT,
  "chat_id" TEXT NOT NULL,
  "message_id" TEXT,
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
