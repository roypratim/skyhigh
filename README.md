# SkyHigh Core – Digital Check-In System

A high-performance backend service for SkyHigh Airlines' digital self-check-in experience. Built with Go, PostgreSQL, and Redis to handle peak-hour traffic with conflict-free seat assignment, time-bound holds, baggage validation, and abuse detection.

---

## Features

| Feature | Implementation |
|---------|---------------|
| Seat hold with 2-min TTL | Redis key `seat_hold:{id}` with 120s expiry |
| Conflict-free seat assignment | Redis `SETNX` distributed lock per seat |
| Auto seat release | Background worker polling every 10 seconds |
| Waitlist promotion | Auto-assigns freed seats to next queued passenger |
| Baggage validation | 25 kg limit; $15/kg excess fee; pauses check-in |
| Payment processing | Simulated payment to resume paused check-in |
| Seat map caching | Redis 30s cache per flight (P95 < 1s) |
| Abuse / bot detection | Sliding-window rate limit (50 maps/2s → 60s block) |

---

## Quick Start (Docker)

**Prerequisites:** Docker and Docker Compose installed.

```bash
# Clone and start
git clone https://github.com/roypratim/skyhigh.git
cd skyhigh
docker-compose up --build
```

The API will be available at `http://localhost:8080`.

On first boot, a demo flight `SH001 JFK→LAX` is seeded with 30 seats and a demo passenger (`demo@skyhigh.io`).

---

## Manual Setup

### Prerequisites

- Go 1.23+
- PostgreSQL 15+
- Redis 7+

### 1. Configure Environment

```bash
cp .env.example .env
# Edit .env with your database and Redis connection details
```

Default values (also used in docker-compose):

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=skyhigh
DB_PASSWORD=skyhigh123
DB_NAME=skyhigh_db
REDIS_HOST=localhost
REDIS_PORT=6379
SERVER_PORT=8080
```

### 2. Set Up the Database

Create the database and user in PostgreSQL:

```sql
CREATE USER skyhigh WITH PASSWORD 'skyhigh123';
CREATE DATABASE skyhigh_db OWNER skyhigh;
```

The application runs **GORM AutoMigrate** on startup—all tables are created automatically. You can also apply the full schema manually:

```bash
psql -U skyhigh -d skyhigh_db -f migrations/001_initial.sql
```

### 3. Run the Application

```bash
go run ./cmd/server
```

### 4. Run Background Workers

Background workers (hold expiry + waitlist promotion) run automatically as goroutines within the main application process. No separate process is required.

---

## Running Tests

```bash
# Run all unit tests
go test ./tests/... -v

# Run with coverage report
go test ./tests/... -v -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

> **Note:** `seat_service_test.go` tests that depend on Redis are skipped automatically when Redis is not available. All other tests run without any external dependencies.

---

## API Overview

Base URL: `http://localhost:8080/api/v1`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/flights` | Create a flight |
| `GET` | `/flights/:id` | Get flight details |
| `GET` | `/flights/:id/seatmap` | Get seat map (rate-limited, cached) |
| `POST` | `/flights/:flightId/seats` | Add seats to a flight |
| `POST` | `/seats/:id/hold` | Hold a seat for 120 seconds |
| `POST` | `/seats/:id/confirm` | Confirm a held seat |
| `POST` | `/checkins` | Start a check-in |
| `GET` | `/checkins/:id` | Get check-in status |
| `DELETE` | `/checkins/:id` | Cancel a check-in |
| `POST` | `/checkins/:id/baggage` | Add baggage to a check-in |
| `POST` | `/checkins/:id/payment` | Process excess baggage payment |
| `POST` | `/flights/:flightId/waitlist` | Join the waitlist |
| `GET` | `/flights/:flightId/waitlist` | View the waitlist |

See [`API-SPECIFICATION.yml`](API-SPECIFICATION.yml) for full OpenAPI 3.0 documentation.

---

## Example Flow

```bash
# 1. Get the seat map for flight 1
curl http://localhost:8080/api/v1/flights/1/seatmap

# 2. Hold seat 3 for passenger 1
curl -X POST http://localhost:8080/api/v1/seats/3/hold \
  -H "Content-Type: application/json" \
  -d '{"passenger_id": 1}'

# 3. Start check-in
curl -X POST http://localhost:8080/api/v1/checkins \
  -H "Content-Type: application/json" \
  -d '{"passenger_id": 1, "flight_id": 1, "seat_id": 3}'

# 4. Add baggage (within limit)
curl -X POST http://localhost:8080/api/v1/checkins/1/baggage \
  -H "Content-Type: application/json" \
  -d '{"weight_kg": 20}'

# 5. Add excess baggage (triggers payment pause)
curl -X POST http://localhost:8080/api/v1/checkins/1/baggage \
  -H "Content-Type: application/json" \
  -d '{"weight_kg": 8}'
# Response: status=WAITING_PAYMENT, excess_fee=45.00

# 6. Process payment
curl -X POST http://localhost:8080/api/v1/checkins/1/payment \
  -H "Content-Type: application/json" \
  -d '{"amount": 45.00}'
```

---

## Project Documents

| Document | Description |
|----------|-------------|
| [`PRD.md`](PRD.md) | Product Requirements Document |
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | System architecture with diagrams |
| [`WORKFLOW_DESIGN.md`](WORKFLOW_DESIGN.md) | Flow diagrams and database schema |
| [`PROJECT_STRUCTURE.md`](PROJECT_STRUCTURE.md) | Codebase structure explained |
| [`API-SPECIFICATION.yml`](API-SPECIFICATION.yml) | OpenAPI 3.0 specification |
| [`CHAT_HISTORY.md`](CHAT_HISTORY.md) | Design decisions and AI collaboration log |

---

## Architecture at a Glance

```
Clients → Load Balancer → SkyHigh Core API (Go/Gin)
                                  │
                    ┌─────────────┴─────────────┐
                    │                           │
               PostgreSQL                    Redis
           (persistent state)         (locks, TTLs, cache,
                                        rate limiting)
```

See [`ARCHITECTURE.md`](ARCHITECTURE.md) for the full diagram.

