package tests

import (
	"testing"

	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBaggageTest(t *testing.T) (*services.BaggageService, models.CheckIn) {
	t.Helper()
	db := newTestDB(t)
	svc := services.NewBaggageService(db)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "baggage_test@test.com")

	ci := models.CheckIn{
		PassengerID: p.ID,
		FlightID:    flight.ID,
		Status:      models.CheckInInProgress,
	}
	require.NoError(t, db.Create(&ci).Error)

	return svc, ci
}

func TestBaggage_WithinLimit(t *testing.T) {
	svc, ci := setupBaggageTest(t)

	baggage, requiresPayment, err := svc.AddBaggage(ci.ID, 20.0)
	require.NoError(t, err)
	assert.False(t, requiresPayment)
	assert.Equal(t, 0.0, baggage.ExcessFee)
	assert.Equal(t, 20.0, baggage.WeightKg)
}

func TestBaggage_ExactLimit(t *testing.T) {
	svc, ci := setupBaggageTest(t)

	baggage, requiresPayment, err := svc.AddBaggage(ci.ID, 25.0)
	require.NoError(t, err)
	assert.False(t, requiresPayment)
	assert.Equal(t, 0.0, baggage.ExcessFee)
}

func TestBaggage_ExceedsLimit(t *testing.T) {
	svc, ci := setupBaggageTest(t)

	// 30 kg → 5 kg excess × $15 = $75
	baggage, requiresPayment, err := svc.AddBaggage(ci.ID, 30.0)
	require.NoError(t, err)
	assert.True(t, requiresPayment)
	assert.Equal(t, 75.0, baggage.ExcessFee)
}

func TestBaggage_MultipleItemsTotalExceedsLimit(t *testing.T) {
	db := newTestDB(t)
	svc := services.NewBaggageService(db)

	flight := seedFlight(t, db)
	p := seedPassenger(t, db, "multi_bag@test.com")
	ci := models.CheckIn{PassengerID: p.ID, FlightID: flight.ID, Status: models.CheckInInProgress}
	require.NoError(t, db.Create(&ci).Error)

	// First bag: 20 kg – within limit
	b1, req1, err := svc.AddBaggage(ci.ID, 20.0)
	require.NoError(t, err)
	assert.False(t, req1)
	assert.Equal(t, 0.0, b1.ExcessFee)

	// Second bag: 10 kg – cumulative 30 kg → 5 kg excess × $15 = $75
	b2, req2, err := svc.AddBaggage(ci.ID, 10.0)
	require.NoError(t, err)
	assert.True(t, req2)
	assert.Equal(t, 75.0, b2.ExcessFee)
}

func TestBaggage_ZeroWeight(t *testing.T) {
	svc, ci := setupBaggageTest(t)

	_, _, err := svc.AddBaggage(ci.ID, 0)
	assert.Error(t, err)
}

func TestBaggage_NegativeWeight(t *testing.T) {
	svc, ci := setupBaggageTest(t)

	_, _, err := svc.AddBaggage(ci.ID, -5.0)
	assert.Error(t, err)
}
