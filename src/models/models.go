package models

type Trip struct {
	ID        int64
	ChatID    int64
	MessageID *int64
	Name      string
}

type Car struct {
	ID            int64
	TripID        int64
	UserID        int64
	Name          string
	MaxPassengers *int64
	Passengers    []Passenger
}

type Passenger struct {
	ID     int64
	CarID  int64
	UserID int64
	Name   string
}

type CarButton struct {
	ID   int64
	Name string
}
