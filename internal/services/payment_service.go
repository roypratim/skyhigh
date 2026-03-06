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
// check-in.  It validates the provided amount against the calculated excess fee,
// creates a Payment record, and resumes the check-in to IN_PROGRESS.
func (s *PaymentService) ProcessPayment(checkInID uint, amount float64) (*models.Payment, error) {
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

	if amount < totalFee {
		return nil, fmt.Errorf("amount %.2f is less than the excess fee owed %.2f", amount, totalFee)
	}

	payment := &models.Payment{
		CheckInID: checkInID,
		Amount:    totalFee,
		Status:    models.PaymentCompleted,
	}
	if err := s.db.Create(payment).Error; err != nil {
		return nil, fmt.Errorf("failed to record payment: %w", err)
	}

	// Resume check-in to IN_PROGRESS so the passenger can complete the process.
	if err := s.db.Model(&models.CheckIn{}).Where("id = ?", checkInID).
		Update("status", models.CheckInInProgress).Error; err != nil {
		return payment, fmt.Errorf("payment recorded but failed to resume check-in: %w", err)
	}

	return payment, nil
}

// GetPaymentsByCheckIn returns all payment records for a check-in.
func (s *PaymentService) GetPaymentsByCheckIn(checkInID uint) ([]models.Payment, error) {
	var payments []models.Payment
	err := s.db.Where("check_in_id = ?", checkInID).Find(&payments).Error
	return payments, err
}
