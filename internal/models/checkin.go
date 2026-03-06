package models

import "time"

// CheckInStatus represents the state of a check-in record.
type CheckInStatus string

const (
	CheckInInProgress     CheckInStatus = "IN_PROGRESS"
	CheckInWaitingPayment CheckInStatus = "WAITING_PAYMENT"
	CheckInCompleted      CheckInStatus = "COMPLETED"
	CheckInCancelled      CheckInStatus = "CANCELLED"
)

// CheckIn records a passenger's check-in for a flight.
type CheckIn struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	PassengerID uint          `gorm:"not null;index" json:"passenger_id"`
	Passenger   *Passenger    `gorm:"foreignKey:PassengerID" json:"passenger,omitempty"`
	FlightID    uint          `gorm:"not null;index" json:"flight_id"`
	Flight      *Flight       `gorm:"foreignKey:FlightID" json:"flight,omitempty"`
	SeatID      *uint         `gorm:"index" json:"seat_id,omitempty"`
	Seat        *Seat         `gorm:"foreignKey:SeatID" json:"seat,omitempty"`
	Status      CheckInStatus `gorm:"size:30;default:IN_PROGRESS" json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}
