package database

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"car_organizer_bot/src/models"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(path string) *Database {
	db, err := sql.Open("sqlite3", filepath.Join(path, "database.sqlite?_journal_mode=WAL&_foreign_keys=on"))
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	return &Database{db}
}

func (d *Database) Close() {
	d.db.Close()
}

func (d *Database) CreateTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS trips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id TEXT NOT NULL,
			message_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS cars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trip_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			max_passengers INTEGER DEFAULT 5,
			FOREIGN KEY(trip_id) REFERENCES trips(id) ON DELETE CASCADE,
			UNIQUE(trip_id, user_id) ON CONFLICT FAIL
		)`,
		`CREATE TABLE IF NOT EXISTS passengers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			car_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			FOREIGN KEY(car_id) REFERENCES cars(id) ON DELETE CASCADE,
			UNIQUE(car_id, user_id) ON CONFLICT FAIL
		)`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			log.Fatalf("failed to create table: %v", err)
		}
	}
}

func (d *Database) InsertTrip(chatID, name string) (int64, error) {
	result, err := d.db.Exec("INSERT INTO trips (chat_id, name) VALUES (?, ?)", chatID, name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (d *Database) UpdateTripMessageID(tripID int64, messageID string) error {
	_, err := d.db.Exec("UPDATE trips SET message_id = ? WHERE id = ?", messageID, tripID)
	return err
}

func (d *Database) SelectTripByID(tripID int64) (*models.Trip, error) {
	row := d.db.QueryRow("SELECT id, chat_id, message_id, name FROM trips WHERE id = ?", tripID)
	t := &models.Trip{}
	if err := row.Scan(&t.ID, &t.ChatID, &t.MessageID, &t.Name); err != nil {
		return nil, err
	}
	return t, nil
}

func (d *Database) AddCar(tripID int64, userID, name string) error {
	_, err := d.db.Exec(
		"INSERT INTO cars (trip_id, user_id, name) VALUES (?, ?, ?)",
		tripID, userID, name,
	)
	return err
}

func (d *Database) RemoveCar(tripID int64, userID string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	carID, err := d.getCarIDTx(tx, tripID, userID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM passengers WHERE car_id = ?", carID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM cars WHERE id = ?", carID); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) UpdateCarSeats(chatID, userID string, maxPassengers int64) (int64, error) {
	row := d.db.QueryRow(`
		SELECT cars.id, cars.trip_id FROM cars
		JOIN trips ON cars.trip_id = trips.id
		WHERE cars.user_id = ? AND trips.chat_id = ?
		ORDER BY cars.id DESC LIMIT 1
	`, userID, chatID)

	var carID, tripID int64
	if err := row.Scan(&carID, &tripID); err != nil {
		return 0, err
	}

	if _, err := d.db.Exec("UPDATE cars SET max_passengers = ? WHERE id = ?", maxPassengers, carID); err != nil {
		return 0, err
	}

	return tripID, nil
}

func (d *Database) AddOrMovePassenger(carID int64, userID, name string) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var tripID int64
	if err := tx.QueryRow("SELECT trip_id FROM cars WHERE id = ?", carID).Scan(&tripID); err != nil {
		return 0, err
	}

	var existingID, existingCarID int64
	err = tx.QueryRow(`
		SELECT p.id, p.car_id FROM passengers p
		JOIN cars c ON p.car_id = c.id
		WHERE c.trip_id = ? AND p.user_id = ?
	`, tripID, userID).Scan(&existingID, &existingCarID)

	if err == nil {
		if existingCarID == carID {
			return 0, fmt.Errorf("already in that car")
		}
		if _, err := tx.Exec("UPDATE passengers SET car_id = ?, name = ? WHERE id = ?", carID, name, existingID); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.Exec(
			"INSERT INTO passengers (car_id, user_id, name) VALUES (?, ?, ?)",
			carID, userID, name,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return tripID, nil
}

func (d *Database) LeaveTrip(tripID int64, userID string) error {
	_, err := d.getCarID(tripID, userID)
	if err == nil {
		return d.RemoveCar(tripID, userID)
	}
	return d.RemovePassenger(tripID, userID)
}

func (d *Database) getCarID(tripID int64, userID string) (int64, error) {
	var id int64
	err := d.db.QueryRow("SELECT id FROM cars WHERE trip_id = ? AND user_id = ?", tripID, userID).Scan(&id)
	return id, err
}

func (d *Database) RemovePassenger(tripID int64, userID string) error {
	_, err := d.db.Exec(`
		DELETE FROM passengers WHERE rowid IN (
			SELECT p.rowid FROM passengers p
			JOIN cars c ON p.car_id = c.id
			WHERE c.trip_id = ? AND p.user_id = ?
		)
	`, tripID, userID)
	return err
}

func (d *Database) UpdateName(userID, name string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE cars SET name = ? WHERE user_id = ?", name, userID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE passengers SET name = ? WHERE user_id = ?", name, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) GetCarsByTrip(tripID int64) ([]models.CarButton, error) {
	rows, err := d.db.Query("SELECT id, name FROM cars WHERE trip_id = ?", tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cars []models.CarButton
	for rows.Next() {
		var c models.CarButton
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		cars = append(cars, c)
	}
	return cars, rows.Err()
}

func (d *Database) BuildTripMessage(tripID int64) (string, error) {
	trip, err := d.SelectTripByID(tripID)
	if err != nil {
		return "", err
	}

	carRows, err := d.db.Query("SELECT id, name, max_passengers FROM cars WHERE trip_id = ? ORDER BY name", tripID)
	if err != nil {
		return "", err
	}
	defer carRows.Close()

	type carInfo struct {
		maxPassengers int64
		passengers    []string
	}
	var carOrder []struct {
		name string
		id   int64
	}
	carMap := make(map[string]*carInfo)

	for carRows.Next() {
		var id int64
		var name string
		var maxPassengers int64
		if err := carRows.Scan(&id, &name, &maxPassengers); err != nil {
			return "", err
		}
		carMap[name] = &carInfo{maxPassengers: maxPassengers}
		carOrder = append(carOrder, struct {
			name string
			id   int64
		}{name, id})
	}
	carRows.Close()

	for _, nc := range carOrder {
		pRows, err := d.db.Query("SELECT name FROM passengers WHERE car_id = ?", nc.id)
		if err != nil {
			return "", err
		}
		for pRows.Next() {
			var username string
			if err := pRows.Scan(&username); err != nil {
				pRows.Close()
				return "", err
			}
			carMap[nc.name].passengers = append(carMap[nc.name].passengers, username)
		}
		pRows.Close()
	}

	text := "📆 <b>" + trip.Name + "</b>\n\n"

	for _, nc := range carOrder {
		info := carMap[nc.name]
		isFull := len(info.passengers) >= int(info.maxPassengers)
		icon := "🚙"
		fullSuffix := ""
		if isFull {
			icon = "🚗"
			fullSuffix = " 🚫"
		}

		text += icon + " <b>" + nc.name + "</b> [" + strconv.Itoa(len(info.passengers)) + "/" + strconv.Itoa(int(info.maxPassengers)) + "]" + fullSuffix + ":\n"
		for _, p := range info.passengers {
			text += "- " + p + "\n"
		}
		text += "\n"
	}

	text += "\n<i>🔄 " + time.Now().Format("15:04:05") + "</i>"

	return text, nil
}

func (d *Database) GetTripsByUserID(userID string) ([]models.Trip, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT trips.id, trips.chat_id, trips.message_id, trips.name
		FROM trips
		JOIN cars ON cars.trip_id = trips.id
		WHERE cars.user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trips []models.Trip
	for rows.Next() {
		var t models.Trip
		if err := rows.Scan(&t.ID, &t.ChatID, &t.MessageID, &t.Name); err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

func (d *Database) getCarIDTx(tx *sql.Tx, tripID int64, userID string) (int64, error) {
	var id int64
	err := tx.QueryRow(
		"SELECT id FROM cars WHERE trip_id = ? AND user_id = ?",
		tripID, userID,
	).Scan(&id)
	return id, err
}
