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
	db      *gorm.DB
	seatSvc *SeatService
}

// NewWaitlistService creates a WaitlistService.
func NewWaitlistService(db *gorm.DB, seatSvc *SeatService) *WaitlistService {
	return &WaitlistService{db: db, seatSvc: seatSvc}
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

	// Determine next position inside a transaction to avoid concurrent duplicates.
	var entry *models.Waitlist
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var maxPos int
		if err := tx.Model(&models.Waitlist{}).
			Where("flight_id = ? AND status = ?", flightID, models.WaitlistWaiting).
			Select("COALESCE(MAX(position), 0)").
			Scan(&maxPos).Error; err != nil {
			return err
		}
		entry = &models.Waitlist{
			FlightID:    flightID,
			PassengerID: passengerID,
			Position:    maxPos + 1,
			Status:      models.WaitlistWaiting,
		}
		return tx.Create(entry).Error
	}); err != nil {
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
// It finds an AVAILABLE seat on the flight, holds it for the passenger via
// SeatService (Redis lock + TTL), and marks the waitlist entry as PROMOTED.
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

	// Find an available seat on this flight.
	var seat models.Seat
	if err := s.db.Where("flight_id = ? AND state = ?", flightID, models.SeatAvailable).
		First(&seat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // no available seat; leave waitlist unchanged
		}
		return fmt.Errorf("failed to find available seat: %w", err)
	}

	// Hold the seat for the promoted passenger (uses Redis distributed lock + TTL).
	// HoldSeat is atomic: it verifies seat state inside the lock, so concurrent
	// PromoteNext calls for the same flight will safely fail on the second attempt.
	if err := s.seatSvc.HoldSeat(seat.ID, entry.PassengerID); err != nil {
		return fmt.Errorf("failed to hold seat for waitlist passenger: %w", err)
	}

	if err := s.db.Model(&entry).Update("status", models.WaitlistPromoted).Error; err != nil {
		// Compensate: reset seat to AVAILABLE so the next worker cycle can retry.
		_ = s.db.Model(&seat).Updates(map[string]interface{}{
			"state":        models.SeatAvailable,
			"passenger_id": nil,
		})
		return fmt.Errorf("failed to promote waitlist entry: %w", err)
	}

	passengerName := "unknown"
	if entry.Passenger != nil {
		passengerName = entry.Passenger.Name
	}
	log.Printf("[WAITLIST] Seat %d held on flight %d – notifying passenger %s (id=%d)",
		seat.ID, flightID, passengerName, entry.PassengerID)

	return nil
}
