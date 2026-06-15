package models

type Trip struct {
	ID        int64
	ChatID    string
	MessageID *string
	Name      string
}

type Car struct {
	ID            int64
	TripID        int64
	UserID        string
	Name          string
	MaxPassengers int64
}

type Passenger struct {
	ID     int64
	CarID  int64
	UserID string
	Name   string
}

type CarButton struct {
	ID   int64
	Name string
}
