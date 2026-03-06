package models

import "time"

// WaitlistStatus describes a passenger's position on the waitlist.
type WaitlistStatus string

const (
	WaitlistWaiting  WaitlistStatus = "WAITING"
	WaitlistPromoted WaitlistStatus = "PROMOTED"
	WaitlistExpired  WaitlistStatus = "EXPIRED"
)

// Waitlist tracks passengers waiting for a seat on a flight.
type Waitlist struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	FlightID    uint           `gorm:"index" json:"flight_id"`
	Flight      *Flight        `gorm:"foreignKey:FlightID" json:"flight,omitempty"`
	PassengerID uint           `gorm:"index" json:"passenger_id"`
	Passenger   *Passenger     `gorm:"foreignKey:PassengerID" json:"passenger,omitempty"`
	Position    int            `json:"position"`
	Status      WaitlistStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
