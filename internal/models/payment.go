package models

import "time"

// PaymentStatus describes the outcome of a payment attempt.
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "PENDING"
	PaymentCompleted PaymentStatus = "COMPLETED"
	PaymentFailed    PaymentStatus = "FAILED"
)

// Payment records a payment associated with a check-in.
type Payment struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	CheckInID uint          `gorm:"index" json:"check_in_id"`
	CheckIn   *CheckIn      `gorm:"foreignKey:CheckInID" json:"check_in,omitempty"`
	Amount    float64       `json:"amount"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}
