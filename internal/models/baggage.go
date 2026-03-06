package models

import "time"

// Baggage represents a piece of luggage added to a check-in.
type Baggage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CheckInID uint      `gorm:"index" json:"check_in_id"`
	CheckIn   *CheckIn  `gorm:"foreignKey:CheckInID" json:"check_in,omitempty"`
	WeightKg  float64   `json:"weight_kg"`
	ExcessFee float64   `json:"excess_fee"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
