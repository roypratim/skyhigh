package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/services"
)

// WaitlistHandler handles waitlist HTTP requests.
type WaitlistHandler struct {
	waitlistSvc *services.WaitlistService
}

// NewWaitlistHandler creates a WaitlistHandler.
func NewWaitlistHandler(svc *services.WaitlistService) *WaitlistHandler {
	return &WaitlistHandler{waitlistSvc: svc}
}

// JoinWaitlist godoc
// POST /api/v1/flights/:flightId/waitlist
func (h *WaitlistHandler) JoinWaitlist(c *gin.Context) {
	flightID, err := strconv.ParseUint(c.Param("flightId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight id"})
		return
	}

	var req struct {
		PassengerID uint `json:"passenger_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry, err := h.waitlistSvc.JoinWaitlist(uint(flightID), req.PassengerID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// GetWaitlist godoc
// GET /api/v1/flights/:flightId/waitlist
func (h *WaitlistHandler) GetWaitlist(c *gin.Context) {
	flightID, err := strconv.ParseUint(c.Param("flightId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight id"})
		return
	}

	entries, err := h.waitlistSvc.GetWaitlist(uint(flightID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"waitlist": entries})
}
