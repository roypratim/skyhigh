package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/roypratim/skyhigh/internal/models"
	"gorm.io/gorm"
)

// FlightHandler handles flight-related HTTP requests.
type FlightHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewFlightHandler creates a FlightHandler.
func NewFlightHandler(db *gorm.DB, rdb *redis.Client) *FlightHandler {
	return &FlightHandler{db: db, redis: rdb}
}

// CreateFlight godoc
// POST /api/v1/flights
func (h *FlightHandler) CreateFlight(c *gin.Context) {
	var req struct {
		FlightNumber  string    `json:"flight_number" binding:"required"`
		Origin        string    `json:"origin" binding:"required"`
		Destination   string    `json:"destination" binding:"required"`
		DepartureTime time.Time `json:"departure_time" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flight := models.Flight{
		FlightNumber:  req.FlightNumber,
		Origin:        req.Origin,
		Destination:   req.Destination,
		DepartureTime: req.DepartureTime,
	}
	if err := h.db.Create(&flight).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, flight)
}

// GetFlight godoc
// GET /api/v1/flights/:flightId
func (h *FlightHandler) GetFlight(c *gin.Context) {
	idStr := c.Param("flightId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight id"})
		return
	}

	var flight models.Flight
	if err := h.db.First(&flight, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "flight not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, flight)
}

// GetSeatMap godoc
// GET /api/v1/flights/:flightId/seatmap
// Results are cached in Redis for 30 seconds.
func (h *FlightHandler) GetSeatMap(c *gin.Context) {
	idStr := c.Param("flightId")
	cacheKey := "seatmap:" + idStr
	ctx := context.Background()

	// Try cache first
	if cached, err := h.redis.Get(ctx, cacheKey).Result(); err == nil {
		var seats []models.Seat
		if json.Unmarshal([]byte(cached), &seats) == nil {
			c.JSON(http.StatusOK, gin.H{"seats": seats, "cached": true})
			return
		}
	}

	flightID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight id"})
		return
	}

	var seats []models.Seat
	if err := h.db.Where("flight_id = ?", flightID).Find(&seats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cache result
	if data, err := json.Marshal(seats); err == nil {
		h.redis.Set(ctx, cacheKey, data, 30*time.Second)
	}

	c.JSON(http.StatusOK, gin.H{"seats": seats, "cached": false})
}
