package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/services"
)

// CheckInHandler handles check-in HTTP requests.
type CheckInHandler struct {
	checkInSvc *services.CheckInService
}

// NewCheckInHandler creates a CheckInHandler.
func NewCheckInHandler(svc *services.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkInSvc: svc}
}

// StartCheckIn godoc
// POST /api/v1/checkins
func (h *CheckInHandler) StartCheckIn(c *gin.Context) {
	var req struct {
		PassengerID uint  `json:"passenger_id" binding:"required"`
		FlightID    uint  `json:"flight_id" binding:"required"`
		SeatID      *uint `json:"seat_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ci, err := h.checkInSvc.StartCheckIn(req.PassengerID, req.FlightID, req.SeatID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ci)
}

// GetCheckIn godoc
// GET /api/v1/checkins/:id
func (h *CheckInHandler) GetCheckIn(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid check-in id"})
		return
	}

	ci, err := h.checkInSvc.GetCheckIn(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "check-in not found"})
		return
	}
	c.JSON(http.StatusOK, ci)
}

// CancelCheckIn godoc
// DELETE /api/v1/checkins/:id
func (h *CheckInHandler) CancelCheckIn(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid check-in id"})
		return
	}

	if err := h.checkInSvc.CancelCheckIn(uint(id)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "check-in cancelled"})
}
