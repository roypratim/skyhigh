package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

const (
	holdTTL       = 120 * time.Second
	lockTTL       = 5 * time.Second
	holdKeyPrefix = "seat_hold:"
	lockKeyPrefix = "lock:seat:"
)

// SeatService handles seat lifecycle operations.
type SeatService struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewSeatService creates a SeatService.
func NewSeatService(db *gorm.DB, rdb *redis.Client) *SeatService {
	return &SeatService{db: db, redis: rdb}
}

func holdKey(seatID uint) string { return fmt.Sprintf("%s%d", holdKeyPrefix, seatID) }
func lockKey(seatID uint) string { return fmt.Sprintf("%s%d", lockKeyPrefix, seatID) }

// acquireLock attempts to obtain a Redis distributed lock.
func (s *SeatService) acquireLock(ctx context.Context, seatID uint) error {
	ok, err := s.redis.SetNX(ctx, lockKey(seatID), "1", lockTTL).Result()
	if err != nil {
		return fmt.Errorf("redis lock error: %w", err)
	}
	if !ok {
		return errors.New("seat is currently being processed, please retry")
	}
	return nil
}

// releaseLock deletes the distributed lock key.
func (s *SeatService) releaseLock(ctx context.Context, seatID uint) {
	s.redis.Del(ctx, lockKey(seatID))
}

// HoldSeat transitions a seat from AVAILABLE → HELD and stores a Redis TTL key.
func (s *SeatService) HoldSeat(seatID, passengerID uint) error {
	ctx := context.Background()

	if err := s.acquireLock(ctx, seatID); err != nil {
		return err
	}
	defer s.releaseLock(ctx, seatID)

	var seat models.Seat
	if err := s.db.First(&seat, seatID).Error; err != nil {
		return fmt.Errorf("seat not found: %w", err)
	}

	if seat.State != models.SeatAvailable {
		return fmt.Errorf("seat is not available (current state: %s)", seat.State)
	}

	// Store hold in Redis with TTL
	if err := s.redis.Set(ctx, holdKey(seatID), passengerID, holdTTL).Err(); err != nil {
		return fmt.Errorf("failed to set redis hold: %w", err)
	}

	// Update DB
	if err := s.db.Model(&seat).Updates(map[string]interface{}{
		"state":        models.SeatHeld,
		"passenger_id": passengerID,
	}).Error; err != nil {
		// Rollback Redis entry on DB failure
		s.redis.Del(ctx, holdKey(seatID))
		return fmt.Errorf("failed to update seat state: %w", err)
	}

	return nil
}

// ConfirmSeat transitions a HELD seat → CONFIRMED for the given passenger.
func (s *SeatService) ConfirmSeat(seatID, passengerID uint) error {
	ctx := context.Background()

	if err := s.acquireLock(ctx, seatID); err != nil {
		return err
	}
	defer s.releaseLock(ctx, seatID)

	var seat models.Seat
	if err := s.db.First(&seat, seatID).Error; err != nil {
		return fmt.Errorf("seat not found: %w", err)
	}

	if seat.State != models.SeatHeld {
		return fmt.Errorf("seat is not in HELD state (current state: %s)", seat.State)
	}

	// Validate hold ownership
	val, err := s.redis.Get(ctx, holdKey(seatID)).Result()
	if err != nil {
		return errors.New("seat hold has expired")
	}
	if val != fmt.Sprintf("%d", passengerID) {
		return errors.New("seat is held by a different passenger")
	}

	if err := s.db.Model(&seat).Update("state", models.SeatConfirmed).Error; err != nil {
		return fmt.Errorf("failed to confirm seat: %w", err)
	}

	// Remove hold key — seat is now permanently confirmed
	s.redis.Del(ctx, holdKey(seatID))
	return nil
}

// ReleaseExpiredHolds finds HELD seats whose Redis hold key has expired and
// resets them to AVAILABLE.  Returns the freed seat IDs for waitlist processing.
func (s *SeatService) ReleaseExpiredHolds() ([]uint, error) {
	ctx := context.Background()

	var heldSeats []models.Seat
	if err := s.db.Where("state = ?", models.SeatHeld).Find(&heldSeats).Error; err != nil {
		return nil, err
	}

	var freed []uint
	for _, seat := range heldSeats {
		exists, err := s.redis.Exists(ctx, holdKey(seat.ID)).Result()
		if err != nil {
			continue
		}
		if exists == 0 {
			// Hold expired — release the seat
			if err := s.db.Model(&seat).Updates(map[string]interface{}{
				"state":        models.SeatAvailable,
				"passenger_id": nil,
			}).Error; err != nil {
				continue
			}
			freed = append(freed, seat.ID)
		}
	}
	return freed, nil
}

// GetSeatsByFlight returns all seats for a flight.
func (s *SeatService) GetSeatsByFlight(flightID uint) ([]models.Seat, error) {
	var seats []models.Seat
	err := s.db.Where("flight_id = ?", flightID).Find(&seats).Error
	return seats, err
}

// CreateSeat persists a new seat record.
func (s *SeatService) CreateSeat(seat *models.Seat) error {
	return s.db.Create(seat).Error
}
