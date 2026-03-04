package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/services"
)

// BaggageHandler handles baggage HTTP requests.
type BaggageHandler struct {
	baggageSvc *services.BaggageService
}

// NewBaggageHandler creates a BaggageHandler.
func NewBaggageHandler(svc *services.BaggageService) *BaggageHandler {
	return &BaggageHandler{baggageSvc: svc}
}

// AddBaggage godoc
// POST /api/v1/checkins/:id/baggage
func (h *BaggageHandler) AddBaggage(c *gin.Context) {
	checkInID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid check-in id"})
		return
	}

	var req struct {
		WeightKg float64 `json:"weight_kg" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baggage, requiresPayment, err := h.baggageSvc.AddBaggage(uint(checkInID), req.WeightKg)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"baggage":          baggage,
		"requires_payment": requiresPayment,
	}
	if requiresPayment {
		resp["message"] = "excess baggage fee applies – check-in paused until payment"
	}
	c.JSON(http.StatusCreated, resp)
}
