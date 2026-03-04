package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/models"
	"github.com/roypratim/skyhigh/internal/services"
)

// SeatHandler handles seat-related HTTP requests.
type SeatHandler struct {
	seatSvc *services.SeatService
}

// NewSeatHandler creates a SeatHandler.
func NewSeatHandler(svc *services.SeatService) *SeatHandler {
	return &SeatHandler{seatSvc: svc}
}

// AddSeats godoc
// POST /api/v1/flights/:flightId/seats
func (h *SeatHandler) AddSeats(c *gin.Context) {
	flightID, err := strconv.ParseUint(c.Param("flightId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight id"})
		return
	}

	var req struct {
		Seats []struct {
			SeatNumber string `json:"seat_number" binding:"required"`
			Class      string `json:"class"`
		} `json:"seats" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var created []models.Seat
	for _, s := range req.Seats {
		class := s.Class
		if class == "" {
			class = "ECONOMY"
		}
		seat := &models.Seat{
			FlightID:   uint(flightID),
			SeatNumber: s.SeatNumber,
			Class:      class,
			State:      models.SeatAvailable,
		}
		if err := h.seatSvc.CreateSeat(seat); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created = append(created, *seat)
	}
	c.JSON(http.StatusCreated, gin.H{"created": len(created), "seats": created})
}

// HoldSeat godoc
// POST /api/v1/seats/:id/hold
func (h *SeatHandler) HoldSeat(c *gin.Context) {
	seatID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seat id"})
		return
	}

	var req struct {
		PassengerID uint `json:"passenger_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.seatSvc.HoldSeat(uint(seatID), req.PassengerID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "seat held successfully", "seat_id": seatID, "ttl_seconds": 120})
}

// ConfirmSeat godoc
// POST /api/v1/seats/:id/confirm
func (h *SeatHandler) ConfirmSeat(c *gin.Context) {
	seatID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seat id"})
		return
	}

	var req struct {
		PassengerID uint `json:"passenger_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.seatSvc.ConfirmSeat(uint(seatID), req.PassengerID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "seat confirmed", "seat_id": seatID})
}
