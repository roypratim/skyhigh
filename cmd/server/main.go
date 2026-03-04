package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/cache"
	"github.com/roypratim/skyhigh/internal/config"
	"github.com/roypratim/skyhigh/internal/db"
	"github.com/roypratim/skyhigh/internal/handlers"
	"github.com/roypratim/skyhigh/internal/middleware"
	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
	"github.com/roypratim/skyhigh/internal/workers"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// --- Infrastructure ---
	database := db.Connect(cfg)
	redisClient := cache.NewRedisClient(cfg)

	// --- Auto-migrate schema ---
	if err := database.AutoMigrate(
		&models.Flight{},
		&models.Passenger{},
		&models.Seat{},
		&models.CheckIn{},
		&models.Baggage{},
		&models.Payment{},
		&models.Waitlist{},
	); err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}

	// --- Seed demo data ---
	seedDemoData(database)

	// --- Services ---
	baggageSvc := services.NewBaggageService(database)
	checkInSvc := services.NewCheckInService(database, baggageSvc)
	seatSvc := services.NewSeatService(database, redisClient)
	paymentSvc := services.NewPaymentService(database, checkInSvc)
	waitlistSvc := services.NewWaitlistService(database)

	// --- Background workers ---
	holdWorker := workers.NewHoldExpiryWorker(database, seatSvc, waitlistSvc)
	waitlistWorker := workers.NewWaitlistWorker(database, waitlistSvc)
	go holdWorker.Run()
	go waitlistWorker.Run()

	// --- Handlers ---
	flightH := handlers.NewFlightHandler(database, redisClient)
	seatH := handlers.NewSeatHandler(seatSvc)
	checkInH := handlers.NewCheckInHandler(checkInSvc)
	baggageH := handlers.NewBaggageHandler(baggageSvc)
	paymentH := handlers.NewPaymentHandler(paymentSvc)
	waitlistH := handlers.NewWaitlistHandler(waitlistSvc)

	// --- Router ---
	router := gin.Default()
	router.Use(gin.Recovery())

	v1 := router.Group("/api/v1")

	// Protected routes (require authentication)
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret))

	// Flights
	// Flight creation is considered an admin-level operation and is protected.
	authGroup.POST("/flights", flightH.CreateFlight)
	// Public read access to flight details and seatmap (seatmap is rate-limited).
	v1.GET("/flights/:id", flightH.GetFlight)
	v1.GET("/flights/:id/seatmap", middleware.RateLimit(redisClient), flightH.GetSeatMap)

	// Seats
	// Seat creation is considered an admin-level operation and is protected.
	authGroup.POST("/flights/:flightId/seats", seatH.AddSeats)
	authGroup.POST("/seats/:id/hold", seatH.HoldSeat)
	authGroup.POST("/seats/:id/confirm", seatH.ConfirmSeat)

	// Check-in
	authGroup.POST("/checkins", checkInH.StartCheckIn)
	authGroup.GET("/checkins/:id", checkInH.GetCheckIn)
	authGroup.DELETE("/checkins/:id", checkInH.CancelCheckIn)

	// Baggage
	authGroup.POST("/checkins/:id/baggage", baggageH.AddBaggage)

	// Payment
	authGroup.POST("/checkins/:id/payment", paymentH.ProcessPayment)

	// Waitlist
	authGroup.POST("/flights/:flightId/waitlist", waitlistH.JoinWaitlist)
	authGroup.GET("/flights/:flightId/waitlist", waitlistH.GetWaitlist)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("SkyHigh Core listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// seedDemoData inserts a demo flight with 30 seats if no flights exist yet.
func seedDemoData(database *gorm.DB) {
	var count int64
	database.Model(&models.Flight{}).Count(&count)
	if count > 0 {
		return
	}

	flight := models.Flight{
		FlightNumber:  "SH001",
		Origin:        "JFK",
		Destination:   "LAX",
		DepartureTime: time.Now().Add(24 * time.Hour),
	}
	if err := database.Create(&flight).Error; err != nil {
		log.Printf("[SEED] failed to create demo flight: %v", err)
		return
	}

	rows := []string{"A", "B", "C", "D", "E", "F"}
	seatNum := 0
	for row := 1; row <= 5 && seatNum < 30; row++ {
		for _, col := range rows {
			if seatNum >= 30 {
				break
			}
			class := "ECONOMY"
			if row <= 2 {
				class = "BUSINESS"
			}
			seat := models.Seat{
				FlightID:   flight.ID,
				SeatNumber: fmt.Sprintf("%d%s", row, col),
				Class:      class,
				State:      models.SeatAvailable,
			}
			database.Create(&seat)
			seatNum++
		}
	}

	// Seed a demo passenger
	passenger := models.Passenger{Name: "Demo User", Email: "demo@skyhigh.io"}
	database.FirstOrCreate(&passenger, models.Passenger{Email: "demo@skyhigh.io"})

	log.Printf("[SEED] demo flight %s created with %d seats (passenger id=%d)", flight.FlightNumber, seatNum, passenger.ID)
}

