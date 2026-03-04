package services

import (
	"errors"
	"fmt"
	"log"

	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

// WaitlistService manages waitlist entries for flights.
type WaitlistService struct {
	db *gorm.DB
}

// NewWaitlistService creates a WaitlistService.
func NewWaitlistService(db *gorm.DB) *WaitlistService {
	return &WaitlistService{db: db}
}

// JoinWaitlist adds a passenger to the waitlist for a flight.
func (s *WaitlistService) JoinWaitlist(flightID, passengerID uint) (*models.Waitlist, error) {
	// Prevent duplicate active waitlist entry
	var existing models.Waitlist
	err := s.db.Where("flight_id = ? AND passenger_id = ? AND status = ?",
		flightID, passengerID, models.WaitlistWaiting).First(&existing).Error
	if err == nil {
		return nil, errors.New("passenger is already on the waitlist for this flight")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Determine next position
	var count int64
	s.db.Model(&models.Waitlist{}).
		Where("flight_id = ? AND status = ?", flightID, models.WaitlistWaiting).
		Count(&count)

	entry := &models.Waitlist{
		FlightID:    flightID,
		PassengerID: passengerID,
		Position:    int(count) + 1,
		Status:      models.WaitlistWaiting,
	}
	if err := s.db.Create(entry).Error; err != nil {
		return nil, fmt.Errorf("failed to join waitlist: %w", err)
	}
	return entry, nil
}

// GetWaitlist returns all WAITING entries for a flight ordered by position.
func (s *WaitlistService) GetWaitlist(flightID uint) ([]models.Waitlist, error) {
	var entries []models.Waitlist
	err := s.db.Preload("Passenger").
		Where("flight_id = ? AND status = ?", flightID, models.WaitlistWaiting).
		Order("position ASC").
		Find(&entries).Error
	return entries, err
}

// PromoteNext promotes the next passenger in the waitlist when a seat opens up.
// It logs a notification (real notification would be email/push).
func (s *WaitlistService) PromoteNext(flightID uint) error {
	var entry models.Waitlist
	err := s.db.Preload("Passenger").
		Where("flight_id = ? AND status = ?", flightID, models.WaitlistWaiting).
		Order("position ASC").
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // no one waiting
	}
	if err != nil {
		return err
	}

	if err := s.db.Model(&entry).Update("status", models.WaitlistPromoted).Error; err != nil {
		return fmt.Errorf("failed to promote waitlist entry: %w", err)
	}

	passengerName := "unknown"
	if entry.Passenger != nil {
		passengerName = entry.Passenger.Name
	}
	log.Printf("[WAITLIST] Seat available on flight %d – notifying passenger %s (id=%d)",
		flightID, passengerName, entry.PassengerID)

	return nil
}
