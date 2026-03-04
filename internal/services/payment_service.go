package services

import (
	"errors"
	"fmt"

	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

// PaymentService simulates payment processing.
type PaymentService struct {
	db         *gorm.DB
	checkInSvc *CheckInService
}

// NewPaymentService creates a PaymentService.
func NewPaymentService(db *gorm.DB, checkInSvc *CheckInService) *PaymentService {
	return &PaymentService{db: db, checkInSvc: checkInSvc}
}

// ProcessPayment simulates a successful payment for the excess baggage fee on a
// check-in.  It creates a Payment record and resumes the check-in.
func (s *PaymentService) ProcessPayment(checkInID uint) (*models.Payment, error) {
	var ci models.CheckIn
	if err := s.db.First(&ci, checkInID).Error; err != nil {
		return nil, fmt.Errorf("check-in not found: %w", err)
	}

	if ci.Status != models.CheckInWaitingPayment {
		return nil, errors.New("check-in is not waiting for payment")
	}

	// Sum pending excess fees for this check-in
	var totalFee float64
	s.db.Model(&models.Baggage{}).
		Where("check_in_id = ?", checkInID).
		Select("COALESCE(SUM(excess_fee), 0)").
		Scan(&totalFee)

	if totalFee <= 0 {
		return nil, errors.New("no excess fee to pay")
	}

	payment := &models.Payment{
		CheckInID: checkInID,
		Amount:    totalFee,
		Status:    models.PaymentCompleted,
	}
	if err := s.db.Create(payment).Error; err != nil {
		return nil, fmt.Errorf("failed to record payment: %w", err)
	}

	// Resume check-in
	if err := s.checkInSvc.CompleteCheckIn(checkInID); err != nil {
		return payment, fmt.Errorf("payment recorded but failed to complete check-in: %w", err)
	}

	return payment, nil
}

// GetPaymentsByCheckIn returns all payment records for a check-in.
func (s *PaymentService) GetPaymentsByCheckIn(checkInID uint) ([]models.Payment, error) {
	var payments []models.Payment
	err := s.db.Where("check_in_id = ?", checkInID).Find(&payments).Error
	return payments, err
}
