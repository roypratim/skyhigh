# CHAT_HISTORY.md – Design Journey with AI Assistant

## Overview

This document chronicles the key design decisions made while building SkyHigh Core, with notes on how the AI assistant was used to evaluate alternatives and guide implementation choices.

---

## Session 1: Requirements Analysis & Architecture Planning

**User:** I need to build a digital check-in system for an airline. Key requirements: seat holds with auto-expiry, conflict-free assignment, waitlist, baggage validation, and abuse detection.

**AI:** Let me break this into the core technical challenges:
1. **Seat conflict prevention** – requires atomic operations; simple DB transactions aren't sufficient under high concurrency
2. **Time-bound holds** – TTL semantics suggest Redis as the natural solution
3. **Seat map performance** – heavy read load → caching layer needed
4. **Abuse detection** – sliding-window rate limiting fits the "50 maps in 2s" pattern

**Decision Point: Technology Stack**

*Alternatives evaluated:*
- **Node.js + PostgreSQL**: Good ecosystem, but GIL-less concurrency in Go is better for the tight lock windows needed
- **Java Spring Boot**: Mature, but heavier startup footprint and less idiomatic for Redis-centric workflows
- **Go + Gin + Redis + PostgreSQL**: Chosen for its lightweight goroutine model, fast startup, and excellent Redis client ecosystem

**Decision:** Go with Gin, GORM (PostgreSQL), and go-redis.

---

## Session 2: Seat Conflict Prevention

**User:** How do I guarantee no two passengers can book the same seat simultaneously?

**AI:** Three approaches considered:

| Approach | Pros | Cons |
|----------|------|------|
| PostgreSQL row-level lock (`SELECT FOR UPDATE`) | No external dependency | Holds DB connection, bottleneck at scale |
| Optimistic locking with version column | Low contention performance | Requires retry logic; complex UX on failure |
| **Redis SETNX distributed lock** | Millisecond TTL, no DB connection held, scales across instances | Requires Redis availability |

**Decision:** Redis SETNX pattern with a 5-second lock TTL. The lock is acquired before reading seat state, preventing any TOCTOU (time-of-check-to-time-of-use) race.

```
SETNX lock:seat:{id} "1" EX 5
→ 1: acquired, proceed
→ 0: another process holds lock, return 409
```

The 5s TTL acts as a safety net against crashes holding the lock indefinitely.

---

## Session 3: Seat Hold Expiry

**User:** How should the 2-minute seat hold auto-expiry work?

**AI:** Options:

| Approach | Description | Verdict |
|----------|-------------|---------|
| **Redis TTL + background poller** | Set `seat_hold:{id}` with EX 120; worker polls for HELD seats with expired keys | ✅ Chosen |
| Redis keyspace notifications | Redis publishes key-expiry events; Go subscribes | Requires `notify-keyspace-events Ex` config; complex in containerised env |
| PostgreSQL scheduled job (pg_cron) | DB-native | Adds operational complexity; mixes concerns |

**Decision:** Redis TTL key (`seat_hold:{id} EX 120`) + Go background goroutine polling every 10 seconds.

The worker queries all HELD seats from PostgreSQL, then checks `EXISTS seat_hold:{id}` for each. Expired holds (key missing) trigger `UPDATE state = AVAILABLE`.

**Why 10s polling instead of 1s?** The hold is 120s, so a 10s poll introduces at most 10s of delay in releasing a seat. This is acceptable and reduces Redis load by 10×.

---

## Session 4: Waitlist Design

**User:** When a seat is released, how should the next waitlisted passenger be notified and assigned?

**AI:** Notification approaches:

| Method | Complexity | Real-time? |
|--------|-----------|-----------|
| Email/SMS via external service | High (third-party) | No (minutes) |
| WebSocket push to client | High (stateful) | Yes |
| **Polling + log notification** | Low | Near-real-time on next poll |
| Redis pub/sub | Medium | Yes |

**Decision for MVP:** When a seat is freed, the `PromoteNext` function assigns the seat to the next waitlisted passenger and logs a notification message. The passenger can poll `GET /checkins/:id` to see their assigned seat.

For production, Redis pub/sub or a WebSocket layer would be added.

**Promotion flow:** `WaitlistService.PromoteNext(flightID, freedSeatID)` → finds lowest-position WAITING entry → holds seat in Redis → updates waitlist status to ASSIGNED → logs.

---

## Session 5: Baggage Validation Logic

**User:** How should the 25kg limit and excess fee work?

**AI:** Design decision: aggregate or per-item check?

- **Per-item check:** Each bag must be ≤ 25kg. Simpler but doesn't reflect real airline policy (total allowance).
- **Aggregate check:** Sum of all bags for the check-in. More realistic.

**Decision:** Aggregate. `BaggageService.AddBaggage` sums existing baggage records for the check-in plus the new item. If total > 25kg, excess = (total - 25) × $15.00.

The check-in status is set to `WAITING_PAYMENT` and a Payment record is inserted with `status = PENDING`. Only after `ProcessPayment` confirms the payment does the check-in return to `IN_PROGRESS`.

**Fee rate:** $15/kg chosen as a typical airline excess baggage rate. Configurable in `baggage_service.go`.

---

## Session 6: Abuse Detection Algorithm

**User:** "One source accessing 50 different seat maps within 2 seconds" – how to detect this?

**AI:** Two sliding-window approaches:

| Approach | Description |
|----------|-------------|
| Fixed window counter | Count requests per second; resets on the second boundary. Simple but can be gamed (burst at boundary) |
| **Sliding window with sorted set** | Redis ZADD with timestamp score; prune old entries; count unique IDs | ✅ Chosen |

**Decision:** Redis sorted set per IP (`seatmap_track:{ip}`):
- Member format: `{flightID}:{timestamp_ns}` (ensures uniqueness per request)
- Score: Unix nanoseconds (enables range-based pruning)
- Prune: `ZREMRANGEBYSCORE -inf {2s_ago}`
- Count unique flight IDs by parsing members
- Threshold: 50 unique IDs → block IP for 60s with `SET block:{ip} "1" EX 60`

The member uniqueness trick (appending nanosecond timestamp) means the same flight ID accessed multiple times is counted once per unique IP+flight pair in the window, correctly capturing "50 different seat maps".

---

## Session 7: Seat Map Caching Strategy

**User:** How do we hit P95 < 1 second for seat map reads?

**AI:** Without caching: a seat map query joins `flights` + `seats` with potentially hundreds of rows. Under 500 concurrent users this can easily exceed 1s.

**Caching strategy:**
- Cache key: `seatmap:{flightID}`
- TTL: **30 seconds** – short enough that passengers see near-real-time availability (a held seat changes within 30s); long enough to absorb peak traffic spikes
- Storage: JSON-encoded seat array in Redis string
- On cache miss: query PostgreSQL, encode, store, return

**Tradeoff:** 30s stale window means a seat that just became HELD might appear AVAILABLE on the map. This is acceptable because the actual hold operation uses a strict distributed lock—a passenger clicking a "AVAILABLE" seat that was just held will get a clear 409 error and see the updated map on next poll.

---

## Session 8: Testing Strategy

**User:** What should we test?

**AI:** Priority matrix:

| Layer | What to test | How |
|-------|-------------|-----|
| BaggageService | Weight aggregation, fee calculation, status transitions | Pure unit tests (no external deps) |
| CheckInService | State transitions, duplicate prevention, cancellation | In-memory DB (sqlite via GORM) |
| SeatService | Hold, confirm, expiry | Requires Redis; skip gracefully when unavailable |
| Handlers | HTTP request/response | Integration test with httptest |

**Decision:** Start with pure unit tests for services (no Redis/PG in CI). Seat service tests skip gracefully when Redis is unavailable (detected by ping error). All business logic is testable without infrastructure.

**Result:** 12 unit tests passing, 5 skipped (Redis-dependent).

---

## Key Architectural Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Concurrency, performance, simplicity |
| HTTP framework | Gin | Low overhead, middleware support |
| ORM | GORM | AutoMigrate, easy querying |
| Primary store | PostgreSQL | ACID, relational integrity |
| Ephemeral store | Redis | TTL, atomic ops, pub/sub |
| Seat locking | Redis SETNX | Prevents race conditions across instances |
| Hold TTL | Redis EX 120 | Natural fit for time-based expiry |
| Seat map cache | Redis 30s TTL | Absorbs read traffic at peak |
| Abuse detection | Redis sorted set sliding window | Accurate per-IP, per-resource counting |
| Deployment | Docker Compose | Single-command reproducible environment |
| Notification | Log-based (MVP) | Defer real notifications to production |
