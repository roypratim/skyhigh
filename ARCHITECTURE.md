# ARCHITECTURE.md – SkyHigh Core

## 1. High-Level Architecture

```
                    ┌─────────────────────────────────────┐
                    │           CLIENT LAYER               │
                    │  (Browsers / Mobile / Kiosk Apps)    │
                    └──────────────────┬──────────────────┘
                                       │ HTTP/REST
                    ┌──────────────────▼──────────────────┐
                    │        API GATEWAY / LOAD BALANCER   │
                    │        (nginx / AWS ALB)             │
                    └──────────────────┬──────────────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
  ┌───────────▼───────────┐ ┌──────────▼──────────┐ ┌──────────▼──────────┐
  │   SkyHigh Core API    │ │  SkyHigh Core API   │ │  SkyHigh Core API   │
  │   (Instance 1)        │ │  (Instance 2)       │ │  (Instance N)       │
  │                       │ │                     │ │                     │
  │  Handlers             │ │  Handlers           │ │  Handlers           │
  │  Services             │ │  Services           │ │  Services           │
  │  Workers (goroutines) │ │  Workers            │ │  Workers            │
  └─────────┬─────────────┘ └──────────┬──────────┘ └──────────┬──────────┘
            │                          │                        │
            └──────────────────────────┼────────────────────────┘
                                       │
                    ┌──────────────────┴──────────────────┐
                    │                                     │
        ┌───────────▼───────────┐         ┌──────────────▼────────────┐
        │      PostgreSQL       │         │          Redis             │
        │  (Persistent Store)   │         │  (Ephemeral / Cache /      │
        │                       │         │   Locking / Rate-Limit)    │
        │  • flights            │         │                            │
        │  • passengers         │         │  • seat_hold:{id} (TTL)    │
        │  • seats              │         │  • lock:seat:{id} (SETNX)  │
        │  • check_ins          │         │  • seatmap:{flightId}      │
        │  • baggage            │         │  • seatmap_track:{ip}      │
        │  • payments           │         │  • block:{ip}              │
        │  • waitlist           │         │                            │
        └───────────────────────┘         └────────────────────────────┘
```

---

## 2. Component Architecture

### 2.1 API Service (`cmd/server/main.go`)

The HTTP server is built with the **Gin** framework. It is **stateless** – all shared state lives in PostgreSQL or Redis, so multiple instances can run behind a load balancer without coordination.

**Startup sequence:**
1. Load config from environment variables
2. Open PostgreSQL connection pool (GORM)
3. Connect to Redis
4. Run `AutoMigrate` to keep schema in sync
5. Seed demo data (idempotent)
6. Instantiate services and handlers
7. Launch background worker goroutines
8. Bind Gin router and listen on `:8080`

---

### 2.2 Service Layer

```
┌────────────────────────────────────────────────────────────────┐
│                        Service Layer                           │
│                                                                │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐  │
│  │  SeatService  │  │CheckInService │  │  BaggageService   │  │
│  │               │  │               │  │                   │  │
│  │ HoldSeat      │  │ StartCheckIn  │  │ AddBaggage        │  │
│  │ ConfirmSeat   │  │ CancelCheckIn │  │ ValidateWeight    │  │
│  │ ReleaseExpired│  │ CompleteCheckIn│ │ CalcExcessFee     │  │
│  └───────┬───────┘  └───────┬───────┘  └────────┬──────────┘  │
│          │                  │                    │             │
│  ┌───────▼───────┐  ┌───────▼───────────────┐   │             │
│  │WaitlistService│  │    PaymentService      │   │             │
│  │               │  │                       │◄──┘             │
│  │ JoinWaitlist  │  │ ProcessPayment        │                  │
│  │ PromoteNext   │  │ (simulated)           │                  │
│  └───────────────┘  └───────────────────────┘                  │
└────────────────────────────────────────────────────────────────┘
```

---

### 2.3 Distributed Locking (Seat Conflict Prevention)

```
Redis SETNX Pattern:

Thread A ──► SETNX lock:seat:42 "1" TTL=5s ──► Result: 1 (OK)
                │ proceed with hold/confirm
                └─► DEL lock:seat:42

Thread B ──► SETNX lock:seat:42 "1" TTL=5s ──► Result: 0 (FAIL)
                │ return 409 Conflict
```

The 5-second lock TTL prevents deadlocks: if Thread A crashes before releasing the lock, it auto-expires in 5 seconds.

---

### 2.4 Seat Hold TTL

```
HoldSeat called:
  SET seat_hold:42 "passengerID" EX 120

  ┌─────────────────────────────────┐
  │  0s      60s      120s          │
  │  │────────────────│             │
  │  key exists       key expires   │
  │  seat = HELD      seat must     │
  │                   return to     │
  │                   AVAILABLE     │
  └─────────────────────────────────┘

HoldExpiryWorker (every 10s):
  SELECT seats WHERE state = 'HELD'
  for each seat:
    EXISTS seat_hold:{id}
    if 0: UPDATE seat SET state = 'AVAILABLE'
          trigger waitlist promotion
```

---

### 2.5 Background Workers

```
main goroutine
    │
    ├── go holdWorker.Run()          (polls every 10s)
    │       │
    │       ├── ReleaseExpiredHolds()
    │       └── PromoteWaitlist for freed seats
    │
    └── go waitlistWorker.Run()      (polls every 30s)
            │
            └── Scan open waitlist entries, assign available seats
```

---

### 2.6 Seat Map Caching

```
GET /api/v1/flights/:id/seatmap

  ┌─────────────────────────────────────────────────────┐
  │  Check Redis: GET seatmap:{flightID}                │
  │                                                     │
  │  Cache HIT ──────────────────► Return JSON (fast)  │
  │                                                     │
  │  Cache MISS                                         │
  │    │                                                │
  │    ├── SELECT seats WHERE flight_id = ?             │
  │    ├── SET seatmap:{flightID} <json> EX 30          │
  │    └── Return JSON                                  │
  └─────────────────────────────────────────────────────┘
```

---

### 2.7 Abuse Detection

```
GET /api/v1/flights/:id/seatmap  ←── RateLimit middleware applied

  ┌────────────────────────────────────────────────────────────┐
  │  1. EXISTS block:{ip}  → if yes, return 429 immediately    │
  │                                                            │
  │  2. ZADD seatmap_track:{ip} score=now member="{id}:ts"     │
  │  3. ZREMRANGEBYSCORE (prune entries older than 2s)          │
  │  4. ZRANGE → extract unique seat IDs from members          │
  │                                                            │
  │  5. if uniqueSeats > 50:                                   │
  │       SET block:{ip} "1" EX 60                             │
  │       log abuse event                                      │
  │       return 429                                           │
  │     else:                                                  │
  │       continue to handler                                  │
  └────────────────────────────────────────────────────────────┘
```

---

## 3. Data Flow: Peak-Hour Check-In

```
500 passengers simultaneously check in:

   P1 ────┐
   P2 ────┤
   P3 ────┤──► Load Balancer ──► API Instance 1 ──┐
   P4 ────┤                                        │
   P5 ────┘                                        │
                                                   │ (each request acquires Redis lock
   P6 ────┐                                        │  for its target seat independently)
   P7 ────┤──► Load Balancer ──► API Instance 2 ──┤
   P8 ────┤                                        │
   ...    ┘                                        │
                                                   ▼
                                          Redis (locks, holds)
                                          PostgreSQL (state)
                                                   │
                                                   ▼
                              Only ONE passenger can hold seat 42
                              All others receive 409 Conflict
```

---

## 4. Deployment Architecture (Docker Compose)

```
┌─────────────────────────────────────────────┐
│              docker-compose.yml             │
│                                             │
│  ┌──────────────┐  ┌──────────┐  ┌───────┐ │
│  │  skyhigh-app │  │ postgres │  │ redis │ │
│  │  :8080       │  │ :5432    │  │ :6379 │ │
│  │              │  │          │  │       │ │
│  │ Go binary    │  │ pg 15    │  │ r7    │ │
│  └──────┬───────┘  └────┬─────┘  └───┬───┘ │
│         │               │            │     │
│         └───────────────┴────────────┘     │
│                  internal network          │
└─────────────────────────────────────────────┘

Start with: docker-compose up --build
```

---

## 5. Technology Choices

| Technology | Reason |
|-----------|--------|
| **Go** | High concurrency, low memory, fast startup; ideal for a distributed service |
| **Gin** | Low-overhead HTTP framework with good middleware ecosystem |
| **PostgreSQL** | ACID-compliant relational DB; ensures durable, consistent seat records |
| **Redis** | Sub-millisecond operations; perfect for TTL holds, locks, and rate-limit counters |
| **GORM** | Reduces boilerplate for DB operations; supports AutoMigrate for schema management |
| **Docker Compose** | Single-command local deployment of all components |

---

## 6. Scalability Considerations

| Concern | Approach |
|---------|---------|
| **Horizontal scaling** | Stateless API instances; shared state in Redis/PG |
| **DB read scalability** | Seat map Redis cache (30s TTL) absorbs most read traffic |
| **Write bottleneck** | Per-seat Redis locks minimise lock contention (no global lock) |
| **Worker duplication** | In multi-instance deployment, the hold-expiry worker should run as a single dedicated process or use Redis leader-election to avoid duplicate releases |
| **Connection pool** | GORM pool (25 max open) per instance; Redis connection managed by go-redis |
