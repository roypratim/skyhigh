package models

import "time"

// SeatState describes the current lifecycle state of a seat.
type SeatState string

const (
	SeatAvailable  SeatState = "AVAILABLE"
	SeatHeld       SeatState = "HELD"
	SeatConfirmed  SeatState = "CONFIRMED"
	SeatCancelled  SeatState = "CANCELLED"
)

// Seat represents a seat on a flight.
type Seat struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	FlightID    uint       `gorm:"not null;index" json:"flight_id"`
	Flight      *Flight    `gorm:"foreignKey:FlightID" json:"flight,omitempty"`
	SeatNumber  string     `gorm:"size:10;not null" json:"seat_number"`
	Class       string     `gorm:"size:20;default:ECONOMY" json:"class"`
	State       SeatState  `gorm:"size:20;default:AVAILABLE" json:"state"`
	PassengerID *uint      `gorm:"index" json:"passenger_id,omitempty"`
	Passenger   *Passenger `gorm:"foreignKey:PassengerID" json:"passenger,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
