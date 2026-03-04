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

	payment, err := h.paymentSvc.ProcessPayment(uint(checkInID))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"payment": payment, "message": "payment processed – check-in completed"})
}
