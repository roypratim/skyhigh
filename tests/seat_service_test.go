package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB returns an in-memory SQLite DB with the schema migrated.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Flight{},
		&models.Passenger{},
		&models.Seat{},
		&models.CheckIn{},
		&models.Baggage{},
		&models.Payment{},
		&models.Waitlist{},
	))
	return db
}

// newTestRedis returns a mock Redis client using a miniredis server.
// If miniredis is not available we skip; tests that need Redis are marked.
func newMockRedis(t *testing.T) *redis.Client {
	t.Helper()
	// Use a real Redis client pointed at an address that will fail gracefully in
	// unit tests – operations are expected to succeed only in integration tests.
	// For unit tests that don't actually call Redis we return a no-op client.
	client := redis.NewClient(&redis.Options{Addr: "localhost:6399"})
	return client
}

func seedFlight(t *testing.T, db *gorm.DB) models.Flight {
	t.Helper()
	f := models.Flight{FlightNumber: "UT001", Origin: "A", Destination: "B"}
	require.NoError(t, db.Create(&f).Error)
	return f
}

func seedPassenger(t *testing.T, db *gorm.DB, email string) models.Passenger {
	t.Helper()
	p := models.Passenger{Name: "Test User", Email: email}
	require.NoError(t, db.Create(&p).Error)
	return p
}

func seedSeat(t *testing.T, db *gorm.DB, flightID uint, number string, state models.SeatState) models.Seat {
	t.Helper()
	s := models.Seat{FlightID: flightID, SeatNumber: number, Class: "ECONOMY", State: state}
	require.NoError(t, db.Create(&s).Error)
	return s
}

// ---------------------------------------------------------------------------
// HoldSeat tests
// ---------------------------------------------------------------------------

func TestHoldSeat_Success(t *testing.T) {
	db := newTestDB(t)

	// Create an in-memory DB + real Redis (skipped if Redis unavailable).
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available, skipping seat hold test")
	}

	svc := services.NewSeatService(db, client)

	flight := seedFlight(t, db)
	passenger := seedPassenger(t, db, "hold_success@test.com")
	seat := seedSeat(t, db, flight.ID, "1A", models.SeatAvailable)

	err := svc.HoldSeat(seat.ID, passenger.ID)
	assert.NoError(t, err)

	var updated models.Seat
	db.First(&updated, seat.ID)
	assert.Equal(t, models.SeatHeld, updated.State)
	assert.Equal(t, &passenger.ID, updated.PassengerID)

	// Cleanup Redis
	client.Del(context.Background(), fmt.Sprintf("seat_hold:%d", seat.ID))
}

func TestHoldSeat_AlreadyHeld(t *testing.T) {
	db := newTestDB(t)
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}

	svc := services.NewSeatService(db, client)
	flight := seedFlight(t, db)
	p1 := seedPassenger(t, db, "held_p1@test.com")
	p2 := seedPassenger(t, db, "held_p2@test.com")
	seat := seedSeat(t, db, flight.ID, "2A", models.SeatAvailable)

	require.NoError(t, svc.HoldSeat(seat.ID, p1.ID))

	err := svc.HoldSeat(seat.ID, p2.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")

	client.Del(context.Background(), fmt.Sprintf("seat_hold:%d", seat.ID))
}

func TestHoldSeat_AlreadyConfirmed(t *testing.T) {
	db := newTestDB(t)
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}

	svc := services.NewSeatService(db, client)
	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "conf@test.com")
	seat := seedSeat(t, db, flight.ID, "3A", models.SeatConfirmed)

	err := svc.HoldSeat(seat.ID, p.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestConfirmSeat_Success(t *testing.T) {
	db := newTestDB(t)
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}

	svc := services.NewSeatService(db, client)
	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "confirm@test.com")
	seat := seedSeat(t, db, flight.ID, "4A", models.SeatAvailable)

	require.NoError(t, svc.HoldSeat(seat.ID, p.ID))

	err := svc.ConfirmSeat(seat.ID, p.ID)
	assert.NoError(t, err)

	var updated models.Seat
	db.First(&updated, seat.ID)
	assert.Equal(t, models.SeatConfirmed, updated.State)
}

func TestSeatHoldExpiry(t *testing.T) {
	db := newTestDB(t)
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}

	svc := services.NewSeatService(db, client)
	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "expiry@test.com")
	seat := seedSeat(t, db, flight.ID, "5A", models.SeatAvailable)

	// Hold the seat
	require.NoError(t, svc.HoldSeat(seat.ID, p.ID))

	// Manually delete the Redis hold to simulate expiry
	client.Del(context.Background(), fmt.Sprintf("seat_hold:%d", seat.ID))

	freed, err := svc.ReleaseExpiredHolds()
	assert.NoError(t, err)
	assert.Contains(t, freed, seat.ID)

	var updated models.Seat
	db.First(&updated, seat.ID)
	assert.Equal(t, models.SeatAvailable, updated.State)
}
