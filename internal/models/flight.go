package models

import "time"

// Flight represents an airline flight.
type Flight struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	FlightNumber  string    `json:"flight_number"`
	Origin        string    `json:"origin"`
	Destination   string    `json:"destination"`
	DepartureTime time.Time `json:"departure_time"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
