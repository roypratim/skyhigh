# WORKFLOW_DESIGN.md – SkyHigh Core

## 1. Primary Flows

### 1.1 Seat Hold Flow

```
Passenger                   API Server              Redis               PostgreSQL
    |                           |                     |                      |
    |--POST /seats/:id/hold---->|                     |                      |
    |                           |--SETNX lock:seat:id--->                   |
    |                           |<---OK (lock acquired)                      |
    |                           |                     |                      |
    |                           |--SELECT seat WHERE id=?------------------>|
    |                           |<--(seat record)----------------------------
    |                           |                     |                      |
    |                           | [check state == AVAILABLE]                 |
    |                           |                     |                      |
    |                           |--SET seat_hold:id passengerID TTL=120s--->|
    |                           |                     |                      |
    |                           |--UPDATE seats SET state=HELD, passenger_id=?-->|
    |                           |                     |                      |
    |                           |--DEL lock:seat:id-->|                      |
    |                           |                     |                      |
    |<--200 OK {seat held}------|                     |                      |
```

**Conflict scenario (two simultaneous requests):**
- Request A: `SETNX lock:seat:5` → returns 1 (acquired), proceeds
- Request B: `SETNX lock:seat:5` → returns 0 (not acquired), returns 409 error
- Only Request A succeeds → **no duplicate holds**

---

### 1.2 Seat Hold Expiry Flow

```
HoldExpiryWorker (every 10s)    Redis               PostgreSQL
        |                          |                     |
        |--SELECT seats WHERE state=HELD-------------->|
        |<--(list of held seats)------------------------
        |                          |                     |
        | for each held seat:      |                     |
        |--EXISTS seat_hold:{id}-->|                     |
        |<--0 (key expired)--------|                     |
        |                          |                     |
        |--UPDATE seats SET state=AVAILABLE, passenger_id=NULL-->|
        |                          |                     |
        | --> trigger waitlist promotion for flight_id   |
```

---

### 1.3 Complete Check-In Flow

```
Passenger                   API Server              PostgreSQL
    |                           |                      |
    |--POST /checkins---------->|                      |
    |  {passenger_id, flight_id,|                      |
    |   seat_id}                |--SELECT seat WHERE id=?-->|
    |                           |<--(seat must be HELD by passenger)
    |                           |                      |
    |                           |--INSERT check_ins--->|
    |                           |--UPDATE seat state=CONFIRMED-->|
    |                           |                      |
    |<--201 {checkin_id, status=IN_PROGRESS}-----------|
    |                           |                      |
    |--POST /checkins/:id/baggage->|                   |
    |  {weight_kg: 30}          |                      |
    |                           | [30 > 25: excess = 5kg × $15 = $75]
    |                           |--INSERT baggage----->|
    |                           |--INSERT payments (amount=$75)-->|
    |                           |--UPDATE check_in status=WAITING_PAYMENT-->|
    |                           |                      |
    |<--200 {status: WAITING_PAYMENT, fee: $75}--------|
    |                           |                      |
    |--POST /checkins/:id/payment->|                   |
    |                           |--UPDATE payment status=CONFIRMED-->|
    |                           |--UPDATE check_in status=IN_PROGRESS-->|
    |<--200 {payment confirmed}--|                      |
    |                           |                      |
    |  (passenger completes)    |                      |
    |--[implicit confirmation]->|                      |
    |                           |--UPDATE check_in status=COMPLETED-->|
    |<--200 {completed}---------|                      |
```

---

### 1.4 Cancellation & Waitlist Promotion Flow

```
Passenger                   API Server              PostgreSQL       Log
    |                           |                      |               |
    |--DELETE /checkins/:id---->|                      |               |
    |                           |--SELECT checkin----->|               |
    |                           |<--(checkin + seat_id)               |
    |                           |                      |               |
    |                           |--UPDATE check_in status=CANCELLED-->|
    |                           |--UPDATE seat state=AVAILABLE, passenger_id=NULL-->|
    |                           |                      |               |
    |<--200 OK------------------|                      |               |
    |                           |                      |               |
    |                           | [waitlist promotion] |               |
    |                           |--SELECT waitlist WHERE flight_id ORDER BY position-->|
    |                           |<--(next passenger)--                |
    |                           |--UPDATE seat state=HELD, passenger_id=next-->|
    |                           |--SET seat_hold:{id} TTL=120s (Redis)|
    |                           |--UPDATE waitlist entry status=ASSIGNED-->|
    |                           |--log "Passenger {id} auto-assigned seat {id}"--->|
```

---

### 1.5 Abuse Detection Flow

```
Bot/Scraper                 API Server              Redis               Log
    |                           |                      |                  |
    | (50 requests in 2s)       |                      |                  |
    |--GET /flights/1/seatmap-->|                      |                  |
    |                           |--EXISTS block:{ip}-->|                  |
    |                           |<--0 (not blocked)    |                  |
    |                           |--ZADD seatmap_track:{ip} score=now member="1:ts"-->|
    |                           |--ZREMRANGEBYSCORE (remove old entries)-->|
    |                           |--ZRANGE seatmap_track:{ip} 0 -1-------->|
    |                           |<--(50 unique seat IDs in window)         |
    |                           |                      |                  |
    |                           | [50 > threshold]     |                  |
    |                           |--SET block:{ip} "1" TTL=60s------------>|
    |                           |--log "IP {ip} blocked: 50 seat maps in 2s"--->|
    |<--429 Too Many Requests---|                      |                  |
    |                           |                      |                  |
    | (next request in 60s)     |                      |                  |
    |--GET /flights/1/seatmap-->|                      |                  |
    |                           |--EXISTS block:{ip}-->|                  |
    |                           |<--1 (blocked)        |                  |
    |<--429 Too Many Requests---|                      |                  |
```

---

## 2. Database Schema

### Entity Relationship Diagram

```
flights                passengers
+---------------+      +------------------+
| id (PK)       |      | id (PK)          |
| flight_number |      | name             |
| origin        |      | email (UNIQUE)   |
| destination   |      | created_at       |
| departure_time|      | updated_at       |
| created_at    |      +------------------+
| updated_at    |             |
+---------------+             |
       |                      |
       |1                     |1
       |                      |
       |N                     |N
       v                      v
    seats               check_ins
+------------------+   +------------------+
| id (PK)          |   | id (PK)          |
| flight_id (FK)   |   | passenger_id (FK)|
| seat_number      |   | flight_id (FK)   |
| class            |   | seat_id (FK)     |
| state            |   | status           |
| passenger_id (FK)|   | created_at       |
| created_at       |   | updated_at       |
| updated_at       |   +------------------+
+------------------+          |
       |                      |1
       |                      |
       |                      |N
       |               +------+------+
       |               |             |
       v               v             v
  waitlist           baggage       payments
+----------------+  +------------+ +------------+
| id (PK)        |  | id (PK)    | | id (PK)    |
| flight_id (FK) |  |check_in_id | |check_in_id |
| passenger_id   |  |(FK)        | |(FK)        |
| position       |  | weight_kg  | | amount     |
| status         |  | excess_fee | | status     |
| created_at     |  | created_at | | created_at |
| updated_at     |  +------------+ | updated_at |
+----------------+                 +------------+
```

### Table Descriptions

| Table | Key Columns | Notes |
|-------|-------------|-------|
| `flights` | `flight_number` (UNIQUE), `origin`, `destination`, `departure_time` | Core flight record |
| `passengers` | `email` (UNIQUE), `name` | Registered travellers |
| `seats` | `flight_id`, `seat_number`, `state`, `passenger_id` | Unique constraint on `(flight_id, seat_number)`. `state` is the lifecycle field. |
| `check_ins` | `passenger_id`, `flight_id`, `seat_id`, `status` | Tracks entire check-in journey |
| `baggage` | `check_in_id`, `weight_kg`, `excess_fee` | One row per bag; service sums across all bags per check-in |
| `payments` | `check_in_id`, `amount`, `status` | One payment per check-in; simulated |
| `waitlist` | `flight_id`, `passenger_id`, `position`, `status` | FIFO queue ordered by `position` |

### State History

Seat state transitions are embedded in the `seats.state` column. Each update to `state` is timestamped via `updated_at`. Check-in lifecycle is captured in `check_ins.status` with `updated_at` tracking the most recent transition.

For full audit trails in production, an append-only `seat_state_history` or `checkin_events` table would be added. The current schema prioritises simplicity while preserving the current state for all operational reads.

---

## 3. Redis Key Design

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `seat_hold:{seatID}` | String | 120 s | Marks a seat as held; expiry auto-releases it |
| `lock:seat:{seatID}` | String | 5 s | Distributed lock preventing concurrent modifications |
| `seatmap:{flightID}` | String (JSON) | 30 s | Cached seat map response |
| `seatmap_track:{ip}` | Sorted Set | 10 s | Tracks recent seat map accesses per IP (sliding window) |
| `block:{ip}` | String | 60 s | Flags a blocked IP after abuse detection |
