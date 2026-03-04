package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	seatMapWindow    = 2 * time.Second
	seatMapThreshold = 50
	blockDuration    = 60 * time.Second
)

// RateLimit returns a Gin middleware that detects abusive seat-map access patterns.
// It uses a Redis sorted set per IP to count unique seat IDs accessed in a 2-second
// sliding window.  IPs exceeding 50 unique seat maps within the window are blocked
// for 60 seconds.
func RateLimit(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := c.Request.Context()

		// Check if IP is already blocked
		blockKey := fmt.Sprintf("block:%s", ip)
		blocked, err := rdb.Exists(ctx, blockKey).Result()
		if err == nil && blocked > 0 {
			log.Printf("[RATELIMIT] Blocked IP %s attempted access", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests – your IP has been temporarily blocked",
			})
			return
		}

		// Only apply sliding-window tracking to seat map endpoints
		seatIDParam := c.Param("id")
		if seatIDParam == "" {
			c.Next()
			return
		}

		trackKey := fmt.Sprintf("seatmap_track:%s", ip)
		now := float64(time.Now().UnixNano())
		member := fmt.Sprintf("%s:%d", seatIDParam, time.Now().UnixNano())

		// Add current access
		rdb.ZAdd(ctx, trackKey, &redis.Z{Score: now, Member: member})
		// Remove entries older than the window
		cutoff := float64(time.Now().Add(-seatMapWindow).UnixNano())
		rdb.ZRemRangeByScore(ctx, trackKey, "-inf", strconv.FormatInt(int64(cutoff), 10))
		// Short TTL to auto-clean the sorted set
		rdb.Expire(ctx, trackKey, 10*time.Second)

		// Count unique seat IDs in the window
		members, err := rdb.ZRange(ctx, trackKey, 0, -1).Result()
		if err == nil {
			uniqueSeats := uniqueSeatIDs(members)
			if uniqueSeats > seatMapThreshold {
				rdb.Set(ctx, blockKey, "1", blockDuration)
				log.Printf("[RATELIMIT] IP %s blocked: accessed %d unique seat maps in %s",
					ip, uniqueSeats, seatMapWindow)
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "abuse detected – your IP has been temporarily blocked",
				})
				return
			}
		}

		c.Next()
	}
}

// uniqueSeatIDs counts how many distinct seat IDs appear in the sorted set members.
// Members are formatted as "{seatID}:{timestamp}".
func uniqueSeatIDs(members []string) int {
	seen := make(map[string]struct{}, len(members))
	for _, m := range members {
		for i := len(m) - 1; i >= 0; i-- {
			if m[i] == ':' {
				seen[m[:i]] = struct{}{}
				break
			}
		}
	}
	return len(seen)
}
