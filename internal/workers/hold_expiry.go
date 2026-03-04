package workers

import (
	"log"
	"time"

	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"gorm.io/gorm"
)

// HoldExpiryWorker periodically scans for HELD seats whose Redis TTL has
// elapsed and resets them to AVAILABLE.
type HoldExpiryWorker struct {
	db          *gorm.DB
	seatSvc     *services.SeatService
	waitlistSvc *services.WaitlistService
	interval    time.Duration
}

// NewHoldExpiryWorker creates the worker.
func NewHoldExpiryWorker(db *gorm.DB, seatSvc *services.SeatService, waitlistSvc *services.WaitlistService) *HoldExpiryWorker {
	return &HoldExpiryWorker{
		db:          db,
		seatSvc:     seatSvc,
		waitlistSvc: waitlistSvc,
		interval:    10 * time.Second,
	}
}

// Run starts the polling loop.  It should be called as a goroutine.
func (w *HoldExpiryWorker) Run() {
	log.Println("[HoldExpiryWorker] started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for range ticker.C {
		w.processExpiredHolds()
	}
}

func (w *HoldExpiryWorker) processExpiredHolds() {
	freedIDs, err := w.seatSvc.ReleaseExpiredHolds()
	if err != nil {
		log.Printf("[HoldExpiryWorker] error releasing holds: %v", err)
		return
	}

	if len(freedIDs) == 0 {
		return
	}

	log.Printf("[HoldExpiryWorker] released %d expired hold(s): %v", len(freedIDs), freedIDs)

	// Collect unique flight IDs for the freed seats
	flightIDs := make(map[uint]struct{})
	for _, seatID := range freedIDs {
		var seat models.Seat
		if err := w.db.Select("flight_id").First(&seat, seatID).Error; err == nil {
			flightIDs[seat.FlightID] = struct{}{}
		}
	}

	// Trigger waitlist processing for each affected flight
	for flightID := range flightIDs {
		if err := w.waitlistSvc.PromoteNext(flightID); err != nil {
			log.Printf("[HoldExpiryWorker] waitlist promote error for flight %d: %v", flightID, err)
		}
	}
}
