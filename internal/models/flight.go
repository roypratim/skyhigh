package models

import "time"

// Flight represents an airline flight.
type Flight struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	FlightNumber  string    `gorm:"uniqueIndex;size:20;not null" json:"flight_number"`
	Origin        string    `gorm:"size:10;not null" json:"origin"`
	Destination   string    `gorm:"size:10;not null" json:"destination"`
	DepartureTime time.Time `gorm:"not null" json:"departure_time"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
