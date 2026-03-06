package services

import (
	"fmt"

	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

const (
	maxBaggageWeightKg = 25.0
	excessFeePerKg     = 15.0 // USD per kg over the limit
)

// BaggageService handles baggage validation and excess-fee calculation.
type BaggageService struct {
	db *gorm.DB
}

// NewBaggageService creates a BaggageService.
func NewBaggageService(db *gorm.DB) *BaggageService {
	return &BaggageService{db: db}
}

// AddBaggage creates a baggage record.  If total weight exceeds the allowance
// the excess fee is computed and the check-in is paused for payment.
// Returns (baggage, requiresPayment, error).
func (s *BaggageService) AddBaggage(checkInID uint, weightKg float64) (*models.Baggage, bool, error) {
	// Validate weight
	if weightKg < 0 {
		return nil, false, fmt.Errorf("baggage weight cannot be negative")
	}
	if weightKg == 0 {
		return nil, false, fmt.Errorf("baggage weight must be greater than zero")
	}

	// Sum existing baggage for this check-in
	var totalWeight float64
	s.db.Model(&models.Baggage{}).
		Where("check_in_id = ?", checkInID).
		Select("COALESCE(SUM(weight_kg), 0)").
		Scan(&totalWeight)

	newTotal := totalWeight + weightKg
	var excessFee float64
	if newTotal > maxBaggageWeightKg {
		excess := newTotal - maxBaggageWeightKg
		excessFee = excess * excessFeePerKg
	}

	baggage := &models.Baggage{
		CheckInID: checkInID,
		WeightKg:  weightKg,
		ExcessFee: excessFee,
	}
	if err := s.db.Create(baggage).Error; err != nil {
		return nil, false, fmt.Errorf("failed to save baggage: %w", err)
	}

	requiresPayment := excessFee > 0
	if requiresPayment {
		// Pause the check-in until payment is made
		if err := s.db.Model(&models.CheckIn{}).Where("id = ?", checkInID).
			Update("status", models.CheckInWaitingPayment).Error; err != nil {
			return baggage, requiresPayment, fmt.Errorf("failed to update check-in status: %w", err)
		}
	}

	return baggage, requiresPayment, nil
}

// GetBaggageByCheckIn returns all baggage items for a check-in.
func (s *BaggageService) GetBaggageByCheckIn(checkInID uint) ([]models.Baggage, error) {
	var items []models.Baggage
	err := s.db.Where("check_in_id = ?", checkInID).Find(&items).Error
	return items, err
}
