package services

import (
	"errors"
	"fmt"

	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

// CheckInService manages the check-in lifecycle.
type CheckInService struct {
	db          *gorm.DB
	baggageSvc  *BaggageService
}

// NewCheckInService creates a CheckInService.
func NewCheckInService(db *gorm.DB, baggageSvc *BaggageService) *CheckInService {
	return &CheckInService{db: db, baggageSvc: baggageSvc}
}

// StartCheckIn creates a new check-in record in IN_PROGRESS state.
func (s *CheckInService) StartCheckIn(passengerID, flightID uint, seatID *uint) (*models.CheckIn, error) {
	// Prevent duplicate active check-ins for same passenger+flight
	var existing models.CheckIn
	err := s.db.Where(
		"passenger_id = ? AND flight_id = ? AND status NOT IN ?",
		passengerID, flightID, []string{string(models.CheckInCancelled), string(models.CheckInCompleted)},
	).First(&existing).Error
	if err == nil {
		return nil, errors.New("active check-in already exists for this passenger and flight")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	ci := &models.CheckIn{
		PassengerID: passengerID,
		FlightID:    flightID,
		SeatID:      seatID,
		Status:      models.CheckInInProgress,
	}
	if err := s.db.Create(ci).Error; err != nil {
		return nil, fmt.Errorf("failed to create check-in: %w", err)
	}
	return ci, nil
}

// GetCheckIn retrieves a check-in by ID with associations.
func (s *CheckInService) GetCheckIn(id uint) (*models.CheckIn, error) {
	var ci models.CheckIn
	err := s.db.Preload("Passenger").Preload("Flight").Preload("Seat").First(&ci, id).Error
	if err != nil {
		return nil, err
	}
	return &ci, nil
}

// CancelCheckIn transitions a check-in to CANCELLED state.
func (s *CheckInService) CancelCheckIn(id uint) error {
	var ci models.CheckIn
	if err := s.db.First(&ci, id).Error; err != nil {
		return fmt.Errorf("check-in not found: %w", err)
	}
	if ci.Status == models.CheckInCompleted || ci.Status == models.CheckInCancelled {
		return fmt.Errorf("cannot cancel a check-in with status %s", ci.Status)
	}
	return s.db.Model(&ci).Update("status", models.CheckInCancelled).Error
}

// CompleteCheckIn marks a check-in as COMPLETED (used after payment if required).
func (s *CheckInService) CompleteCheckIn(id uint) error {
	var ci models.CheckIn
	if err := s.db.First(&ci, id).Error; err != nil {
		return fmt.Errorf("check-in not found: %w", err)
	}
	if ci.Status != models.CheckInInProgress && ci.Status != models.CheckInWaitingPayment {
		return fmt.Errorf("check-in cannot be completed from status %s", ci.Status)
	}
	return s.db.Model(&ci).Update("status", models.CheckInCompleted).Error
}

// PauseForPayment sets a check-in to WAITING_PAYMENT.
func (s *CheckInService) PauseForPayment(id uint) error {
	return s.db.Model(&models.CheckIn{}).Where("id = ?", id).
		Update("status", models.CheckInWaitingPayment).Error
}
