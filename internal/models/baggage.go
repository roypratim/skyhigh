package models

import "time"

// Baggage represents a piece of luggage added to a check-in.
type Baggage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CheckInID  uint      `gorm:"not null;index" json:"check_in_id"`
	CheckIn    *CheckIn  `gorm:"foreignKey:CheckInID" json:"check_in,omitempty"`
	WeightKg   float64   `gorm:"type:decimal(5,2);not null" json:"weight_kg"`
	ExcessFee  float64   `gorm:"type:decimal(10,2);default:0" json:"excess_fee"`
	CreatedAt  time.Time `json:"created_at"`
}
