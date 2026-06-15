package database

import (
	"os"
	"strings"
	"testing"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	dir, err := os.MkdirTemp("", "car_organizer_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	db := NewDatabase(dir)
	db.CreateTables()
	return db
}

func TestInsertTripAndSelectByID(t *testing.T) {
	db := newTestDB(t)

	id, err := db.InsertTrip("C123", "Beach Trip")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero trip id")
	}

	trip, err := db.SelectTripByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if trip.Name != "Beach Trip" {
		t.Fatalf("expected 'Beach Trip', got '%s'", trip.Name)
	}
	if trip.ChatID != "C123" {
		t.Fatalf("expected 'C123', got '%s'", trip.ChatID)
	}
}

func TestAddCarAndRemoveCar(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	if err := db.AddCar(tripID, "U1", "Alice"); err != nil {
		t.Fatal(err)
	}

	if err := db.AddCar(tripID, "U1", "Alice"); err == nil {
		t.Fatal("expected error for duplicate car")
	}

	cars, err := db.GetCarsByTrip(tripID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cars) != 1 {
		t.Fatalf("expected 1 car, got %d", len(cars))
	}

	if err := db.RemoveCar(tripID, "U1"); err != nil {
		t.Fatal(err)
	}

	cars, _ = db.GetCarsByTrip(tripID)
	if len(cars) != 0 {
		t.Fatalf("expected 0 cars after removal, got %d", len(cars))
	}
}

func TestUpdateCarSeats(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	db.AddCar(tripID, "U1", "Alice")

	retTripID, err := db.UpdateCarSeats("C123", "U1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if retTripID != tripID {
		t.Fatalf("expected tripID %d, got %d", tripID, retTripID)
	}

	msg, err := db.BuildTripMessage(tripID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "3") {
		t.Fatalf("expected seats count 3 in message, got: %s", msg)
	}
}

func TestAddOrMovePassenger(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddCar(tripID, "U2", "Bob")

	cars, _ := db.GetCarsByTrip(tripID)
	aliceCarID := cars[0].ID
	bobCarID := cars[1].ID

	// Join Alice's car
	tripID2, err := db.AddOrMovePassenger(aliceCarID, "U3", "Charlie")
	if err != nil {
		t.Fatal(err)
	}
	if tripID2 != tripID {
		t.Fatalf("expected tripID %d, got %d", tripID, tripID2)
	}

	// Move to Bob's car
	tripID3, err := db.AddOrMovePassenger(bobCarID, "U3", "Charlie")
	if err != nil {
		t.Fatal(err)
	}
	if tripID3 != tripID {
		t.Fatalf("expected tripID %d, got %d", tripID, tripID3)
	}

	// Already in Bob's car
	_, err = db.AddOrMovePassenger(bobCarID, "U3", "Charlie")
	if err == nil {
		t.Fatal("expected error for already in that car")
	}
	if !strings.Contains(err.Error(), "already in that car") {
		t.Fatalf("expected 'already in that car', got: %v", err)
	}
}

func TestLeaveTripCarOwner(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddCar(tripID, "U2", "Bob")
	db.AddOrMovePassenger(getFirstCarID(t, db, tripID), "U3", "Charlie")

	// Alice (car owner) leaves — car and all its passengers should be removed
	if err := db.LeaveTrip(tripID, "U1"); err != nil {
		t.Fatal(err)
	}

	cars, _ := db.GetCarsByTrip(tripID)
	if len(cars) != 1 {
		t.Fatalf("expected 1 car after owner leave, got %d", len(cars))
	}
	if cars[0].Name != "Bob" {
		t.Fatalf("expected Bob's car to remain, got %s", cars[0].Name)
	}
}

func TestLeaveTripPassenger(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddOrMovePassenger(getFirstCarID(t, db, tripID), "U2", "Bob")

	// Bob (passenger) leaves
	if err := db.LeaveTrip(tripID, "U2"); err != nil {
		t.Fatal(err)
	}

	// Car should still exist, Bob should be gone
	msg, _ := db.BuildTripMessage(tripID)
	if strings.Contains(msg, "Bob") {
		t.Fatalf("expected Bob to be removed, message: %s", msg)
	}
}

func TestUpdateName(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddOrMovePassenger(getFirstCarID(t, db, tripID), "U2", "Bob")

	if err := db.UpdateName("U1", "Alicia"); err != nil {
		t.Fatal(err)
	}

	msg, err := db.BuildTripMessage(tripID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Alicia") {
		t.Fatalf("expected updates name 'Alicia' in message, got: %s", msg)
	}
	if strings.Contains(msg, "Alice") {
		t.Fatalf("expected old name 'Alice' to be gone, message: %s", msg)
	}
}

func TestGetTripsByUserID(t *testing.T) {
	db := newTestDB(t)

	trip1, _ := db.InsertTrip("C123", "Trip 1")
	trip2, _ := db.InsertTrip("C456", "Trip 2")
	db.AddCar(trip1, "U1", "Alice")
	db.AddCar(trip2, "U1", "Alice")

	trips, err := db.GetTripsByUserID("U1")
	if err != nil {
		t.Fatal(err)
	}
	if len(trips) != 2 {
		t.Fatalf("expected 2 trips, got %d", len(trips))
	}
}

func TestBuildTripMessageFormat(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Fun Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddOrMovePassenger(getFirstCarID(t, db, tripID), "U2", "Bob")

	msg, err := db.BuildTripMessage(tripID)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(msg, "Fun Trip") {
		t.Fatalf("expected trip name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Alice") {
		t.Fatalf("expected driver name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Bob") {
		t.Fatalf("expected passenger name in message, got: %s", msg)
	}
	if !strings.Contains(msg, "/") {
		t.Fatalf("expected seats count (X/Y) in message, got: %s", msg)
	}
	if !strings.Contains(msg, "<i>") {
		t.Fatalf("expected timestamp in message, got: %s", msg)
	}
}

func TestBuildTripMessageEmptyCar(t *testing.T) {
	db := newTestDB(t)

	tripID, _ := db.InsertTrip("C123", "Empty Trip")
	db.AddCar(tripID, "U1", "Alice")
	db.AddCar(tripID, "U2", "Bob")

	msg, err := db.BuildTripMessage(tripID)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(msg, "Alice") {
		t.Fatalf("expected Alice in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Bob") {
		t.Fatalf("expected Bob in message, got: %s", msg)
	}
	if strings.Contains(msg, "-") {
		t.Fatalf("expected no passenger items for empty cars, message: %s", msg)
	}
}

func getFirstCarID(t *testing.T, db *Database, tripID int64) int64 {
	t.Helper()
	cars, err := db.GetCarsByTrip(tripID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cars) == 0 {
		t.Fatal("no cars found")
	}
	return cars[0].ID
}
