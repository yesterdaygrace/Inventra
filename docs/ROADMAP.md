# Inventra Execution Roadmap

Derived from `task.md` v3.1 (Final Design Baseline) and the grilling-session decision log.
Status legend: `[x]` done · `[~]` partially done · `[ ]` not started.

## Decisions (locked)

- Scope: P0 verification + full P1. P2 deferred (ABC, demand planning, suppliers, unit conversion, purchase orders).
- Execution follows PRD §77 phase order. Frontend screens land with each phase.
- API aligns to PRD §43/§44: `/inventory/receive|issue|adjust|transfer`, `{data, meta}` envelope, stable error codes.
- Four roles: ADMIN, WAREHOUSE_MANAGER, STAFF, VIEWER.
- Demo data wiped and reseeded to showcase every feature (mixed-expiry batches, dead stock, count variance).
- New `inventory_ledger` table per §15; running balance computed on read (window function), never stored.
- Costing: per-product `costing_method`, default `WEIGHTED_AVERAGE`; FIFO consumes oldest cost layers first.
- Products carry `track_batches` + `expiry_tracked` flags; FEFO engages only when `expiry_tracked`.
- Adjustments: PENDING by default; auto-approved when performer has `inventory.adjust` AND |value| < settings threshold ($500 default); else WAREHOUSE_MANAGER queue.
- Cycle counts: warehouse-scoped plans holding an explicit SKU list; ad-hoc = plan of one.
- Reservations expire lazily (evaluated on read / on issue), released in the observing transaction.
- Reorder: `available < reorder_point` ⇒ recommend `max(max_stock − available, safety_stock)`.
- Tests: unit tests with each feature; concurrency + integration suites land in their phase.

## Phase 0 — Baseline Verification `[x]`

- [x] Migrations framework (7 versioned pairs, embedded)
- [x] Modular monolith layout (internal/* modules, shared kernel)
- [x] Row-level locking on stock mutations (`clause.Locking{Strength:"UPDATE"}` ×4 in inventory repo)
- [x] Idempotency middleware + store + tests (24h TTL, hashed keys)
- [x] Refresh-token family table
- [x] Full `go test -p 1 ./...` green — 19/19 packages (2026-08-25)
- [x] Gap list vs P0 checklist:
  - Ledger per §15 missing (transactions ≠ ledger shape) → Phase 2
  - RBAC has 2 roles, PRD needs 4 → Phase 1
  - Refresh reuse detection implemented but unverified against revocation test → Phase 1 audit
  - Concurrency scenario test (100/70/50) absent → Phase 3

Note: backend integration tests require the `crudin-postgres` container (host port 5433,
db `inventory`). It had died with its volume; recreated via `CREATE DATABASE inventory`.

## Phase 1 — Foundation Completion `[x]`

- [x] Roles: WAREHOUSE_MANAGER + VIEWER added (migration 000008 widens both
      `roles_name_check` and the GORM-mirrored `chk_roles_name`; seed grants
      ADMIN 20 / WM 15 / STAFF 10 / VIEWER 6 permissions)
- [x] Permission catalog aligned to PRD §41: `inventory.receive|issue|adjust`,
      `warehouse.manage`, `user.manage`, `audit.read`, `report.read|export`;
      routers updated (report routes gained enforcement they lacked)
- [x] Request-ID verified end-to-end (middleware → Zap logger → audit context → activity rows)
- [x] Health endpoints `/healthz` + `/ready` (DB ping) confirmed present

Gotchas recorded: Makefile `SEED_DB` prefix overrides caller env (`DB_PORT=5434 make seed`
still hits 5433 — pass full env or edit SEED_DB); golang-migrate records the version
before running, so a failed migration needs `schema_migrations` reset, not just dirty-flag.

## Phase 2 — Inventory Core (PRD alignment) `[x]`

- [x] `inventory_ledger` table per §15 (migration 000009; 7 transaction types,
      transfer pairing, batch_id reserved); `inventory_transactions` retired
- [x] Endpoints renamed: `/inventory/receive`, `/issue`, `/transfers`;
      ledger read model at GET `/inventory/ledger` with running balance
      computed via window function (never stored)
- [x] Response envelope `{data, meta}` + stable §67 error codes across ALL
      modules (response package v2; handler signatures unchanged)
- [x] Reservations (migration 000010): create/release/consume/list under
      `/inventory/reservations`; lazy expiry releases quantity in the first
      transaction that observes it; Issue + Transfer now check availability
      as on-hand − active-reserved; consume writes an ISSUE ledger entry
      referencing the reservation
- [x] Frontend: api.ts parses `{data, meta}` + `{error:{code,message}}`,
      renamed endpoints wired; internal `ListResult.pagination` name kept so
      page components were untouched

Note: `/adjust` intentionally deferred to Phase 4 — writing ADJUSTMENT rows
without the approval workflow would violate PRD §23 (no direct stock edits).

## Phase 3 — Correctness `[x]`

- [x] PRD §69 mandatory concurrency scenario: stock=100, concurrent issue
      70 + 50 → exactly one succeeds, one conflicts, final = 30-or-50,
      exactly one ledger row — iterated 20× (`TestConcurrentStockOutLeavesNoNegative`)
- [x] Concurrent transfers keep warehouse sums consistent, one transfer_id
      pair recorded (`TestConcurrentTransfersKeepSumsConsistent`)
- [x] Transfer atomicity: overdraw rollback, unknown-warehouse 404,
      per-warehouse isolation (`TestTransfer*` family)
- [x] Duplicate-request guarantee end-to-end: same Idempotency-Key replayed
      against POST /inventory/receive returns the byte-identical stored
      response and exactly one movement; same key + different body → 409
      DUPLICATE_REQUEST (`TestReceiveIdempotencyNoDuplicateMovements`)
- [x] All inventory integration tests run on real PostgreSQL (5433)

Hardening added along the way: Issue + Transfer check availability as
on-hand − active-reserved; lazy reservation expiry runs inside the locking
transaction; `setupForRepo` test harness is now self-cleaning.

## Phase 4 — Inventory Control

- [ ] Adjustment requests: reasons enum (§23), evidence notes, PENDING→approved/rejected
- [ ] Approval queue UI for WAREHOUSE_MANAGER; before/after preview (§57)
- [ ] Cycle counts: plans, count entry (Save & Next UX §56), variance calc, approval → adjustment → ledger
- [ ] Frontend: Adjustments page, Cycle Counting page

## Phase 5 — Inventory Intelligence

- [ ] Product flags `track_batches`, `expiry_tracked`, `costing_method`
- [ ] Batch model + receive-with-batch flow; expiry statuses NORMAL/EXPIRING_SOON/EXPIRED
- [ ] FEFO deterministic issuing for expiry_tracked products (+ unit tests)
- [ ] Costing engine: WAC + FIFO over cost layers; COGS on issues (+ PRD worked-example tests)
- [ ] Valuation views: total / by warehouse / category / batch
- [ ] Stock aging buckets (0-30/31-60/61-90/91-180/180+); dead-stock detection
- [ ] Frontend: Batches & Expiry page, Ledger page (financial style §53), Valuation, Aging, Dead Stock

## Phase 6 — Planning (P1 slice only)

- [ ] Reorder engine + recommendations endpoint + action-queue UI (§59)
- [ ] Analytics endpoints: turnover, stock health, movement volumes (§38)
- [ ] Dashboard rebuild per §49/§50: attention-first layout, actionable warnings
- [ ] Navigation regrouped per §48

## Phase 7 — Production Quality

- [ ] E2E happy-path suite (login → receive → reserve → issue → transfer → count → approve → ledger)
- [ ] CI pipeline: lint, unit, integration, build (coverage target noted, not gating early phases)
- [ ] Seed rewrite: multi-warehouse, mixed-expiry batches, dead stock, pending variance
- [ ] Swagger regeneration; README refresh

## Explicitly deferred (P2)

ABC analysis · demand planning (moving average / weighted / exponential smoothing) · unit conversion · suppliers · purchase replenishment · OpenTelemetry/metrics evaluation.
