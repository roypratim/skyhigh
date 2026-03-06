package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roypratim/skyhigh/internal/services"
)

// PaymentHandler handles payment HTTP requests.
type PaymentHandler struct {
	paymentSvc *services.PaymentService
}

// NewPaymentHandler creates a PaymentHandler.
func NewPaymentHandler(svc *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentSvc: svc}
}

// ProcessPayment godoc
// POST /api/v1/checkins/:id/payment
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	checkInID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid check-in id"})
		return
	}

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.paymentSvc.ProcessPayment(uint(checkInID), req.Amount)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "payment confirmed", "check_in_status": "IN_PROGRESS"})
}
