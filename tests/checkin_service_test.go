package tests

import (
	"testing"

	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartCheckIn_Success(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "checkin_ok@test.com")

	ci, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, models.CheckInInProgress, ci.Status)
	assert.Equal(t, p.ID, ci.PassengerID)
	assert.Equal(t, flight.ID, ci.FlightID)
}

func TestStartCheckIn_DuplicatePrevented(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "checkin_dup@test.com")

	_, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)

	_, err = svc.StartCheckIn(p.ID, flight.ID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCancelCheckIn(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "cancel_ci@test.com")

	ci, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)

	err = svc.CancelCheckIn(ci.ID)
	assert.NoError(t, err)

	updated, _ := svc.GetCheckIn(ci.ID)
	assert.Equal(t, models.CheckInCancelled, updated.Status)
}

func TestCancelCheckIn_AlreadyCancelled(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "cancel_twice@test.com")

	ci, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)
	require.NoError(t, svc.CancelCheckIn(ci.ID))

	err = svc.CancelCheckIn(ci.ID)
	assert.Error(t, err)
}

func TestCheckIn_PauseOnExcessBaggage(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "excess_ci@test.com")

	ci, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)

	// Add 30 kg – exceeds 25 kg limit
	_, requiresPayment, err := baggageSvc.AddBaggage(ci.ID, 30.0)
	require.NoError(t, err)
	assert.True(t, requiresPayment)

	updated, _ := svc.GetCheckIn(ci.ID)
	assert.Equal(t, models.CheckInWaitingPayment, updated.Status)
}

func TestCompleteCheckIn(t *testing.T) {
	db := newTestDB(t)
	baggageSvc := services.NewBaggageService(db)
	svc := services.NewCheckInService(db, baggageSvc, nil)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "complete_ci@test.com")

	ci, err := svc.StartCheckIn(p.ID, flight.ID, nil)
	require.NoError(t, err)

	err = svc.CompleteCheckIn(ci.ID)
	assert.NoError(t, err)

	updated, _ := svc.GetCheckIn(ci.ID)
	assert.Equal(t, models.CheckInCompleted, updated.Status)
}
