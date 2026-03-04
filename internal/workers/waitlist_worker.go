package workers

import (
	"log"
	"time"

	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"gorm.io/gorm"
)

// WaitlistWorker periodically checks for flights that have available seats and
// promotes the next passenger on the waitlist.
type WaitlistWorker struct {
	db          *gorm.DB
	waitlistSvc *services.WaitlistService
	interval    time.Duration
}

// NewWaitlistWorker creates the worker.
func NewWaitlistWorker(db *gorm.DB, waitlistSvc *services.WaitlistService) *WaitlistWorker {
	return &WaitlistWorker{
		db:          db,
		waitlistSvc: waitlistSvc,
		interval:    30 * time.Second,
	}
}

// Run starts the polling loop.  It should be called as a goroutine.
func (w *WaitlistWorker) Run() {
	log.Println("[WaitlistWorker] started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for range ticker.C {
		w.processWaitlists()
	}
}

func (w *WaitlistWorker) processWaitlists() {
	// Find all flights that have AVAILABLE seats.
	var flightIDs []uint
	if err := w.db.Model(&models.Seat{}).
		Select("DISTINCT flight_id").
		Where("state = ?", models.SeatAvailable).
		Pluck("flight_id", &flightIDs).Error; err != nil {
		log.Printf("[WaitlistWorker] error querying available seats: %v", err)
		return
	}

	for _, flightID := range flightIDs {
		var count int64
		if err := w.db.Model(&models.Waitlist{}).
			Where("flight_id = ? AND status = ?", flightID, models.WaitlistWaiting).
			Count(&count).Error; err != nil {
			log.Printf("[WaitlistWorker] error counting waitlist for flight %d: %v", flightID, err)
			continue
		}
		if count > 0 {
			if err := w.waitlistSvc.PromoteNext(flightID); err != nil {
				log.Printf("[WaitlistWorker] error promoting for flight %d: %v", flightID, err)
			}
		}
	}
}
