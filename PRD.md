# PRD: SkyHigh Core – Digital Check-In System

## 1. Overview

SkyHigh Core is the backend service powering SkyHigh Airlines' digital self-check-in experience. It is designed to handle high-concurrency peak-hour traffic—hundreds of passengers simultaneously selecting seats, adding baggage, and completing check-in—with correctness, speed, and resilience.

---

## 2. Problem Statement

During popular flight check-in windows, hundreds of passengers attempt to:
- Browse the seat map
- Select and reserve seats
- Add baggage
- Complete check-in

Traditional synchronous approaches fail under this load due to seat conflicts (multiple passengers booking the same seat), slow seat map queries, and lack of automated seat lifecycle management.

SkyHigh Core solves these problems with a distributed, Redis-backed architecture that guarantees conflict-free seat assignment, time-bounded holds, and near-real-time seat availability.

---

## 3. Goals

| Goal | Description |
|------|-------------|
| No seat overlaps | Exactly one passenger can hold or confirm any seat at a time |
| Fast check-in | Seat map browsing P95 latency < 1 second |
| Baggage validation | Enforce 25 kg limit; pause and collect fees for excess |
| Auto seat expiry | Unreleased HELD seats automatically expire after 2 minutes |
| Waitlist support | Passengers can queue for full flights; auto-assigned when a seat frees |
| Abuse protection | Detect and block bot/scraper traffic patterns |
| Audit trail | All abuse events and state transitions are logged |

---

## 4. Key Users

| User | Description |
|------|-------------|
| **Passenger** | A traveller who selects a seat, adds baggage, and completes check-in |
| **Admin** | Airline staff who create flights and add seats to the system |
| **System** | Background workers that expire holds and promote waitlisted passengers |

---

## 5. Core Functional Requirements

### 5.1 Seat Availability & Lifecycle Management

Each seat follows a defined state machine:

```
AVAILABLE → HELD → CONFIRMED → CANCELLED
              ↓
          (expires after 120s)
              ↓
          AVAILABLE
```

| State | Meaning |
|-------|---------|
| `AVAILABLE` | Seat can be viewed and selected |
| `HELD` | Temporarily reserved for one passenger for 120 seconds |
| `CONFIRMED` | Permanently assigned to a passenger |
| `CANCELLED` | Released after cancellation; becomes AVAILABLE |

**Business Rules:**
- A seat can only be `HELD` if it is currently `AVAILABLE`
- Only one passenger may hold a seat at any time
- `CONFIRMED` seats may be `CANCELLED` by the passenger

### 5.2 Time-Bound Seat Hold (2 Minutes)

- When a passenger holds a seat, the system reserves it for exactly **120 seconds**
- A Redis key `seat_hold:{seatID}` with 120s TTL tracks the hold
- If the passenger does not confirm within 120 seconds, the seat automatically returns to `AVAILABLE`
- A background worker polls every 10 seconds, finds HELD seats with no corresponding Redis key, and releases them

### 5.3 Conflict-Free Seat Assignment

- A Redis distributed lock (`lock:seat:{seatID}`) with 5-second TTL is acquired before any hold/confirm operation
- Only the first requester acquires the lock; all others receive an error
- This guarantee holds regardless of concurrent request volume

### 5.4 Cancellation

- Passengers may cancel a `CONFIRMED` check-in before departure
- The associated seat transitions from `CONFIRMED` → `AVAILABLE`
- Waitlisted passengers for the flight are automatically promoted

### 5.5 Waitlist Assignment

- If no seats are available, passengers may join the waitlist for a flight
- When a seat becomes `AVAILABLE` (cancellation or expired hold), the system automatically assigns it to the next eligible waitlisted passenger
- The promotion is logged as a notification

### 5.6 Baggage Validation & Payment Pause

- Maximum allowed baggage weight: **25 kg**
- If total baggage weight exceeds 25 kg:
  - Check-in is paused with status `WAITING_PAYMENT`
  - Excess fee: **$15.00 per kg** over the limit
  - A payment record is created for the passenger
- Only after a `CONFIRMED` payment may the passenger complete check-in
- Check-in statuses: `IN_PROGRESS` | `WAITING_PAYMENT` | `COMPLETED` | `CANCELLED`

### 5.7 High-Performance Seat Map Access

- Seat map data is cached in Redis per flight with a **30-second TTL**
- On cache miss the map is read from PostgreSQL and repopulated
- This ensures P95 seat map latency < 1 second under concurrent load

### 5.8 Abuse & Bot Detection

- Each seat map request is tracked in a Redis sorted set keyed by the requesting IP
- If a single IP accesses **50 distinct seat maps within 2 seconds**, the IP is blocked for **60 seconds**
- Blocked requests receive HTTP 429
- Each block event is logged for audit

---

## 6. Non-Functional Requirements (NFRs)

| Category | Requirement |
|----------|-------------|
| **Performance** | Seat map API P95 latency < 1 second under 500 concurrent users |
| **Consistency** | No duplicate seat assignments under any concurrency level |
| **Availability** | Service must handle Redis or DB connection errors gracefully (fail-safe) |
| **Scalability** | Stateless HTTP service; multiple instances can run behind a load balancer |
| **Security** | API endpoints require passenger authentication (JWT bearer token) |
| **Observability** | Structured logging for all state transitions and abuse events |
| **Durability** | PostgreSQL used for persistent storage; Redis used for ephemeral state only |
| **Portability** | Full system deployable via a single `docker-compose up` command |
| **Testability** | Unit tests covering all business-logic services with ≥ 80% statement coverage |

---

## 7. Out of Scope

- Real payment gateway integration (payment is simulated)
- Email/SMS notification infrastructure (notifications are logged)
- Boarding pass generation
- Loyalty/frequent flyer program
- Multi-region geo-distribution
