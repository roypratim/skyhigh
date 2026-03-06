# PROJECT_STRUCTURE.md – SkyHigh Core

## Top-Level Layout

```
skyhigh/
├── cmd/                    # Application entry points
├── internal/               # Private application packages
├── migrations/             # Raw SQL schema migrations
├── tests/                  # Unit test suite
├── docker-compose.yml      # Full-stack local deployment
├── Dockerfile              # Container image definition
├── go.mod / go.sum         # Go module dependencies
├── .env.example            # Environment variable reference
└── docs/                   # Required deliverable documents
    ├── PRD.md
    ├── README.md
    ├── PROJECT_STRUCTURE.md   (this file)
    ├── WORKFLOW_DESIGN.md
    ├── ARCHITECTURE.md
    ├── API-SPECIFICATION.yml
    └── CHAT_HISTORY.md
```

---

## `cmd/server/`

| File | Purpose |
|------|---------|
| `main.go` | Application entry point. Initialises PostgreSQL, Redis, all services, registers Gin routes, starts background workers, and seeds demo data on first boot. |

---

## `internal/config/`

| File | Purpose |
|------|---------|
| `config.go` | Reads all configuration from environment variables. Provides sensible defaults so the app runs locally without any extra setup. Key fields: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `REDIS_HOST`, `REDIS_PORT`, `SERVER_PORT`. |

---

## `internal/db/`

| File | Purpose |
|------|---------|
| `postgres.go` | Opens and validates a GORM PostgreSQL connection. Configures a connection pool (25 max open, 10 max idle, 1-hour lifetime). Returns `*gorm.DB` for use by all services. |

---

## `internal/cache/`

| File | Purpose |
|------|---------|
| `redis.go` | Creates and validates a `go-redis` client. Used for: distributed locks, seat-hold TTLs, seat-map caching, rate-limit sorted sets, and IP block flags. |

---

## `internal/models/`

GORM models that map to PostgreSQL tables.

| File | Table | Purpose |
|------|-------|---------|
| `flight.go` | `flights` | Stores flight metadata: number, origin, destination, departure time. |
| `seat.go` | `seats` | Each seat on a flight. Tracks `state` (AVAILABLE/HELD/CONFIRMED/CANCELLED) and `passenger_id`. |
| `passenger.go` | `passengers` | Registered travellers with name and email. |
| `checkin.go` | `check_ins` | Links a passenger to a flight/seat. Tracks check-in `status`. |
| `baggage.go` | `baggage` | Baggage records per check-in. Stores `weight_kg` and computed `excess_fee`. |
| `payment.go` | `payments` | Payment records for excess-baggage fees. Tracks `amount` and `status`. |
| `waitlist.go` | `waitlist` | Queue of passengers waiting for a seat on a flight. Tracks `position` and `status`. |

---

## `internal/services/`

Business logic layer. Each service is injected with a `*gorm.DB` and optionally a `*redis.Client`.

| File | Purpose |
|------|---------|
| `seat_service.go` | Core seat lifecycle: `HoldSeat`, `ConfirmSeat`, `ReleaseExpiredHolds`. Uses Redis `SETNX` distributed lock (`lock:seat:{id}`) and Redis TTL (`seat_hold:{id}`) to guarantee conflict-free assignment and auto-expiry. |
| `checkin_service.go` | `StartCheckIn`, `GetCheckIn`, `CancelCheckIn`, `CompleteCheckIn`. Manages check-in status transitions. On cancel, the associated seat is freed and the waitlist is checked. |
| `baggage_service.go` | `AddBaggage`. Validates total weight across all bags. If total > 25 kg, calculates excess fee at $15/kg, creates a Payment record, and sets check-in status to `WAITING_PAYMENT`. |
| `payment_service.go` | `ProcessPayment`. Simulates payment confirmation. On success, transitions the check-in back to `IN_PROGRESS` so the passenger can complete it. |
| `waitlist_service.go` | `JoinWaitlist`, `GetWaitlist`, `PromoteNext`. When a seat is freed (expiry or cancellation), `PromoteNext` assigns it to the top-ranked waitlisted passenger and logs a notification. |

---

## `internal/handlers/`

Gin HTTP handlers. Each file corresponds to one resource group and delegates to the service layer.

| File | Endpoints |
|------|-----------|
| `flight.go` | `POST /api/v1/flights`, `GET /api/v1/flights/:id`, `GET /api/v1/flights/:id/seatmap` (cached) |
| `seat.go` | `POST /api/v1/flights/:flightId/seats`, `POST /api/v1/seats/:id/hold`, `POST /api/v1/seats/:id/confirm` |
| `checkin.go` | `POST /api/v1/checkins`, `GET /api/v1/checkins/:id`, `DELETE /api/v1/checkins/:id` |
| `baggage.go` | `POST /api/v1/checkins/:id/baggage` |
| `payment.go` | `POST /api/v1/checkins/:id/payment` |
| `waitlist.go` | `POST /api/v1/flights/:flightId/waitlist`, `GET /api/v1/flights/:flightId/waitlist` |

---

## `internal/middleware/`

| File | Purpose |
|------|---------|
| `ratelimit.go` | Abuse/bot detection. Uses a Redis sorted set (`seatmap_track:{ip}`) with a 2-second sliding window. Blocks IPs that access > 50 unique seat maps within the window for 60 seconds. Returns HTTP 429. All block events are logged. |
| `auth.go` | Placeholder JWT authentication middleware for future use. Currently passes all requests through for developer convenience. |

---

## `internal/workers/`

Background goroutines started from `main.go`.

| File | Interval | Purpose |
|------|----------|---------|
| `hold_expiry.go` | 10 s | Finds seats in `HELD` state whose Redis TTL key (`seat_hold:{id}`) no longer exists. Resets them to `AVAILABLE`. After releasing, triggers waitlist promotion. |
| `waitlist_worker.go` | 30 s | Scans all open waitlist entries and attempts to assign available seats on the corresponding flight. Handles cases where the hold_expiry worker has not yet promoted. |

---

## `migrations/`

| File | Purpose |
|------|---------|
| `001_initial.sql` | Complete PostgreSQL DDL for all seven tables. Includes foreign-key constraints, indexes, and default values. The application also uses GORM `AutoMigrate` on startup for convenience. |

---

## `tests/`

Unit tests using Go's standard `testing` package with `github.com/stretchr/testify`.

| File | Coverage |
|------|---------|
| `seat_service_test.go` | HoldSeat (success, already held, already confirmed), ConfirmSeat, hold expiry simulation (skipped when Redis is unavailable) |
| `checkin_service_test.go` | StartCheckIn, duplicate prevention, CancelCheckIn, pause on excess baggage, CompleteCheckIn |
| `baggage_service_test.go` | Within-limit, exact-limit, exceeds-limit, multi-bag total, zero weight, negative weight |

Run with:
```bash
go test ./tests/... -v -cover
```
