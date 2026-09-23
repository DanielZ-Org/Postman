# Delivery Office — Canonical Specification

Status: **initial design specification / pre-MVP**

This document records the decisions made for the first Delivery Office implementation. It is intended to be usable by both developers and coding agents. Do not silently invent missing game rules. Items explicitly marked **OPEN** or **DEFERRED** are not yet canonical.

---

## 1. Technical architecture

### 1.1 Stack

- Backend: **Go**
- Frontend: **React + TypeScript**
- API: **REST + JSON**
- Initial API namespace: `/api/v1`
- Database: **SQLite**
- Deployment during initial development: **local only**
- Real-time WebSockets/SSE: **not required for MVP**

The backend owns authoritative game state, gameplay rules, rule validation, state transitions and calculations. The UI sends commands/choices and renders the state returned by the backend.

### 1.2 Compatibility rule

API evolution should be additive wherever possible.

Safe example:

```json
{
  "cash": 500,
  "interest_due": 50
}
```

Adding `interest_due` later should not break a client that only consumes `cash`.

Breaking changes include removing fields, renaming fields, changing field types, or changing established semantics. Those require an explicit migration/versioning decision.

### 1.3 Security baseline

Initial backend should:

- bind to `127.0.0.1`;
- validate incoming payloads;
- never trust calculations supplied by the UI;
- configure CORS deliberately for the local frontend;
- avoid exposing the database directly;
- keep authoritative state and game logic backend-side.

No anti-cheat system is required for the local single-player MVP.

### 1.4 State machines and rule ownership

Objects with an operational lifecycle should use explicit backend-owned states and controlled transitions.

Examples:

- package lifecycle: `stored → assigned → out_for_delivery → delivered`;
- walking employee lifecycle: `ready → packing → out_for_delivery → ready`.

A requested action must be validated against gameplay rules before a state transition is accepted. Invalid transitions must be rejected by the backend rather than corrected or inferred by the frontend.

The frontend may display state and request actions, but it must not duplicate authoritative gameplay rules.

Future gameplay events may temporarily modify normal rules. Event modifiers are applied before validation, but they do **not** bypass lifecycle/state-machine constraints.

Canonical evaluation order:

```text
Base rules
→ Active event modifiers
→ Rule validation
→ State transition
→ API response
```

The external/random event system remains **DEFERRED** for the MVP; this section only defines the extension boundary so it can be added later without restructuring the core backend.

### 1.5 Backend lifecycle, startup bootstrap and persistence ownership

The backend owns authoritative game state and gameplay rules. The React frontend is untrusted input: it sends commands and choices and renders the state returned by the backend.

- API boundary remains versioned REST/JSON under `/api/v1`.
- Initial deployment binds to localhost only (`127.0.0.1`).
- SQLite remains the initial persistence mechanism.

#### New-game bootstrap (M1)

Game state is **not** created as a side effect of `GET /api/v1/game`.

When the backend process starts:

- if a persisted game exists, load it;
- otherwise create the default initial M1 game state in memory.

The initial game time remains the canonical value defined by this specification.

`GET` endpoints remain strictly read-only and must not mutate or seed authoritative state.

An explicit "new game" reset endpoint is deferred until the product needs reset / multiple-save / new-game UI semantics, and is **not** part of M1.

---

# 2. Game clock and calendar

## 2.1 Starting date

New games begin:

**1 February 1980**

This is fictional game time. It is not tied to UTC, the host clock or the player's timezone.

## 2.2 Time scale

A full 24-hour game day lasts:

| Speed | Real duration / game day |
|---|---:|
| Pause | no advancement |
| 1x | 12 minutes |
| 2x | 6 minutes |
| 3x | 4 minutes |

At 1x, one real second therefore advances two game minutes.

## 2.3 Offline behaviour

The simulation advances **only while the game is running**.

If the player closes the game for seven real days, the game resumes at exactly the saved game time. Packages, bills and deliveries do not advance while the game is closed.

Use elapsed/monotonic runtime while playing rather than deriving progress from the computer's BIOS/system clock.

## 2.4 Working hours

| Day | Opening hours |
|---|---|
| Monday–Friday | 09:00–17:00 |
| Saturday | 10:00–13:00 |
| Sunday | Closed |

This is 43 scheduled opening hours per week.

The world clock still covers 24 hours. Future systems such as drones may operate outside office opening hours.

The UI should provide **Skip to next opening** when appropriate. Skipping advances the game calendar rather than simply changing a display value. Any future background systems active during skipped time must be processed correctly.

## 2.5 Calendar events

The backend must be able to detect at least:

- new game day;
- Tuesday payroll;
- Friday rent payment;
- four-week financial periods.

An internal event architecture may be added later; an external event API is not required for the first slice.

### Example clock state

```json
{
  "game_datetime": "1980-02-01T09:00:00",
  "day_of_week": "friday",
  "speed": 1,
  "paused": false,
  "office_open": true,
  "days_until_next_payroll": 4,
  "days_until_next_rent": 0
}
```

---

# 3. Player

A new player begins with a **£1,000 loan**, so initial cash is £1,000 and initial loan principal is also £1,000.

The player may optionally have a logo and avatar/face reference. The backend only needs asset references/IDs initially; the UI can own actual image assets.

## 3.1 Player traits

One player trait may be selected.

| Trait | Effect |
|---|---|
| Financial | +10% revenue bonus, applied to aggregate revenue at end-of-day settlement |
| Storage | +10% usable storage capacity |
| Logistics | +10% delivery capacity |

Trait effects belong in backend calculations.

The financial bonus should **not** mutate every individual package's base payout. Calculate ordinary package revenue first, aggregate it, then apply the 10% player modifier at settlement.

### Example player

```json
{
  "id": "player-1",
  "name": "Daniel",
  "logo_id": "logo-01",
  "avatar_id": "avatar-01",
  "trait": "financial",
  "cash": 650.0,
  "loan_principal": 1000.0,
  "head_office_id": "office-small-01"
}
```

---

# 4. Office / warehouse

The first selected location is the player's **head office**. The model should allow future branch locations without redesigning the existing head-office object.

## 4.1 Initial office choices

| Property | Small | Large |
|---|---:|---:|
| Down payment | £350 | £450 |
| Weekly rent | £50 | £75 |
| Initial storage capacity | 100 units | 150 units |
| Maximum upgraded storage | 150 units | 250 units |
| Employee capacity | 5 | 7 |
| Bicycle capacity | 5 | 7 |
| Vehicle/garage capacity | 1 | 2 |
| Accepted package sizes initially | small, medium | small, medium |

The down payment includes the **first four weeks** of occupancy. Weekly Friday rent begins after that prepaid period.

Large packages require a future office upgrade before they can be accepted.

Decorations, equipment upgrades and capacity upgrades are **DEFERRED**, but the maximum-capacity fields should exist now.

### Example office

```json
{
  "id": "office-small-01",
  "type": "small",
  "is_head_office": true,
  "down_payment": 350.0,
  "weekly_rent": 50.0,
  "rent_prepaid_weeks": 4,
  "next_rent_due": "1980-02-29T00:00:00",
  "storage": {
    "base_capacity": 100,
    "current_capacity": 100,
    "maximum_capacity": 150,
    "used_units": 0
  },
  "employee_capacity": 5,
  "bicycle_capacity": 5,
  "vehicle_capacity": 1,
  "accepted_package_sizes": ["small", "medium"],
  "contract_status": "active",
  "missed_rent_payments": 0
}
```

## 4.2 Missed rent

- First missed rent payment: add a **20% late fee once**.
- Second missed rent payment: terminate the office contract.
- If the player has no valid office and cannot afford to enter a new office contract, the game ends.

The precise recovery/grace-period UX is **OPEN**.

## 4.3 Office selection (M1)

### Selectable offices

The M1 selectable office set is exactly two, in this deterministic order:

| ID | Type | Down payment | Weekly rent | Rent prepaid | Storage base/current at selection | Storage max | Employee capacity | Bicycle capacity | Vehicle capacity | Accepted package sizes initially |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `office-small-01` | small | £350 | £50 | 4 weeks | 100 | 150 | 5 | 5 | 1 | small, medium |
| `office-large-01` | large | £450 | £75 | 4 weeks | 150 | 250 | 7 | 7 | 2 | small, medium |

There are no other selectable offices in M1. Office definitions are immutable backend-owned catalogue data; clients cannot alter them.

### Starting cash

Canonical starting player cash is **£1000**. Money is represented using **integer pounds** for M1 (no floating point; pennies/pence are not yet required). The authoritative cash balance belongs to backend game state.

### `GET /api/v1/offices`

Returns the office **options/catalogue definitions** (not mutable selected-office state), with HTTP `200 OK`, `Content-Type: application/json`, under a single `offices` wrapper in deterministic order (`office-small-01` then `office-large-01`). Catalogue objects intentionally do NOT contain runtime fields (`is_head_office`, `next_rent_due`, `storage.current`, `storage.used`, `contract_status`, `missed_rent_payments`) — those are created when an office is selected. GET is read-only.

```json
{
  "offices": [
    {
      "id": "office-small-01",
      "type": "small",
      "down_payment": 350,
      "weekly_rent": 50,
      "rent_prepaid_weeks": 4,
      "storage": { "base": 100, "max": 150 },
      "employee_capacity": 5,
      "bicycle_capacity": 5,
      "vehicle_capacity": 1,
      "accepted_package_sizes": ["small", "medium"]
    }
  ]
}
```

### `POST /api/v1/offices/select` request

```json
{ "office_id": "office-small-01" }
```

Field `office_id` (string) is required. Unknown fields, malformed JSON, and multiple/trailing JSON values are rejected.

### One-time selection

M1 permits exactly **one** head-office selection. Before selection the selected office is `nil`. After a successful selection that office becomes the head office; a second office cannot be selected, and selecting the same office again is also rejected. Branch offices are outside M1D scope.

### Successful selection state

A successfully selected office becomes a runtime Office instance following the §4.1 representation (adds `is_head_office`, `next_rent_due`, `storage.current`/`storage.used`, `contract_status: "active"`, `missed_rent_payments: 0`). `next_rent_due` follows the existing rule: the down payment covers `rent_prepaid_weeks` weeks, so weekly rent is first due that many weeks after the selection date at midnight (e.g., selected Feb 1 → `1980-02-29T00:00:00`).

### Successful selection response

HTTP `200 OK`, `Content-Type: application/json`:

```json
{
  "office": { "...": "canonical selected runtime office object" },
  "cash_balance": 650
}
```

From the canonical £1000 starting cash, selecting Small yields `cash_balance = 650`; selecting Large yields `cash_balance = 550`. The response does not return the entire game state.

### Atomic selection behavior

A successful selection is one atomic operation: (1) validate no office is selected; (2) resolve the requested canonical definition; (3) validate sufficient cash; (4) construct the runtime Office; (5) deduct the exact down payment; (6) append exactly one finance transaction; (7) commit all state changes together. If any validation fails, the selected office, cash, and transaction list are all left unchanged — no partial mutation.

### Minimal finance invariant

Per §11.3, monetary mutations create a finance transaction. M1D introduces only the minimum finance state to preserve that invariant (current cash balance + an in-memory transaction collection) — not the general finance subsystem. A successful selection appends exactly one transaction:

```json
{
  "type": "office_down_payment",
  "amount": -350,
  "game_datetime": "1980-02-01T09:00:00",
  "balance_after": 650,
  "office_id": "office-small-01"
}
```

`amount` is the negative down payment (Small `-350`, Large `-450`). This transaction is internal game state; M1D adds no `/api/v1/finance` endpoints, transaction IDs, persistence, payroll, rent charging, late fees, loans, or generic finance commands.

### Selection timestamp

The office down-payment transaction uses the authoritative fictional Clock snapshot at the instant selection executes. It never uses `time.Now()` / host wall clock, and office selection does not advance or otherwise mutate game time.

### Office selection error codes

All errors use the canonical envelope from §14.1 (`error.code`, `error.message`, optional `error.details`). Deterministic business validation order: (1) malformed/invalid request transport; (2) already-selected state; (3) office existence; (4) sufficient funds.

| Code | HTTP | Meaning |
|---|---:|---|
| `OFFICE_NOT_FOUND` | 404 | `office_id` is syntactically valid but does not identify a canonical selectable office; state unchanged |
| `INSUFFICIENT_FUNDS` | 409 | the requested office exists but current cash is less than its down payment; state unchanged (`details` may carry `{"required":N,"available":M}`) |
| `OFFICE_ALREADY_SELECTED` | 409 | a head office is already selected (same or other office); any syntactically valid selection request returns this without changing state |

### Generic API errors and unsupported methods

The generic codes also apply: `INVALID_JSON` (400) for malformed JSON / multiple values / undecodable body; `INVALID_REQUEST` (400) for missing `office_id`, wrong type, empty office ID, unknown fields, or otherwise-invalid shape. `GET /api/v1/offices` accepts GET only and `POST /api/v1/offices/select` accepts POST only; other methods return HTTP 405 with `METHOD_NOT_ALLOWED` (JSON) and must not mutate state. No plain-text errors on Office API routes.

---

# 5. Packages

## 5.1 Package sizes

Sizes:

- `small`
- `medium`
- `large`

Storage consumption:

| Size | Storage units |
|---|---:|
| Small | 1 |
| Medium | 1 |
| Large | 2 |

## 5.2 Service type and deadline

Service types:

- `normal`
- `express`

Deadlines:

| Service | Deadline |
|---|---|
| Normal | 5 game days after receipt |
| Express | 2 game days after receipt |

Capacity consumption during a delivery run:

- normal package = 1 delivery-capacity unit;
- express package = 2 delivery-capacity units.

Express therefore consumes extra delivery capacity, but still counts as one package for employee wage calculation.

## 5.3 Package revenue

Known prices:

| Size | Normal | Express |
|---|---:|---:|
| Small | £5 | £12 |
| Medium | £7 | £15 |
| Large | **OPEN** | **OPEN** |

Late-delivery payout penalties:

- normal: **25% reduction**;
- express: **75% reduction**.

Example: a £12 express package delivered late produces £3 base revenue before player-level modifiers.

## 5.4 Destination

Destination types:

- `local`
- `far`

MVP packages are all `local`.

Future rule:

- walking and bicycles: local delivery only;
- cars: local and far delivery.

Far destinations are **DEFERRED** for the first vertical slice.

## 5.5 Status and state transitions

Initial package state enum:

- `stored`
- `assigned`
- `out_for_delivery`
- `delivered`

Canonical initial lifecycle:

`stored → assigned → out_for_delivery → delivered`

The backend is authoritative for package state and legal transitions. A package must not jump directly between non-adjacent states unless a future explicitly defined rule or recovery flow allows it.

Additional failure/cancellation states can be added later.

### Example package

```json
{
  "id": "pkg-000001",
  "size": "small",
  "service_type": "normal",
  "destination_type": "local",
  "storage_units": 1,
  "delivery_capacity_units": 1,
  "base_fee": 5.0,
  "received_at": "1980-02-01T09:00:00",
  "due_at": "1980-02-06T09:00:00",
  "status": "stored",
  "assigned_employee_id": null,
  "delivered_at": null,
  "final_revenue": null
}
```

---

# 6. Package generation

Initial arrival rate:

**2 packages per working/open hour.**

This hourly rate is the canonical rule. Do not derive or hard-code a separate fixed daily package count. Package generation occurs only during office opening hours.

Express packages:

- first four weeks / first game month: **0% express**;
- afterwards: **3% chance** for a generated package to be express.

Initial generated package destinations are local.

The size probability distribution between small and medium is **OPEN**.

If warehouse capacity is insufficient, the behaviour is **OPEN**. Do not silently overfill storage.

---

# 7. Employees

Employees have names but no gender field is required.

## 7.1 Employee speed trait

Initial family/archetype labels:

- `snail`
- `chicken`
- `cheetah`

These represent employee speed/performance characteristics.

Exact numeric modifiers are **OPEN**.

## 7.2 Skills

Skills are represented as an extensible list.

Initial capability progression:

- no transport skill;
- bicycle skill;
- bicycle + driving licence;
- future drone licence.

Training mechanics are **DEFERRED**. Initially the player may hire employees who already possess skills.

## 7.3 Mood

Mood enum:

- `happy`
- `neutral`
- `unhappy`

Default: `neutral`.

Mood mechanics and resignation behaviour are **DEFERRED**, although future retention should matter because hiring cost uses a historical high-water mark.

## 7.4 Delivery state

Each delivery employee has an operational state:

- `ready` — available to be assigned a delivery cycle.
- `packing` — assigned packages are being prepared; the employee cannot accept another assignment.
- `out_for_delivery` — currently performing the delivery portion of the cycle and cannot accept another assignment.

The backend owns this state and the legal transitions between states. The frontend may request an assignment, but it must not directly change employee operational state.

For a walking cycle the canonical transition is:

`ready → packing → out_for_delivery → ready`

A requested assignment is valid only if the relevant rules pass, including employee availability, package eligibility and capacity. Future event modifiers may alter rule values, but they do not permit an otherwise illegal employee-state transition.

## 7.5 Employee performance

Track packages delivered by each employee during the current payroll period/week.

This is an actual per-employee counter, not merely an aggregate company statistic.

### Example employee

```json
{
  "id": "emp-0001",
  "name": "Bob Snail",
  "speed_trait": "snail",
  "skills": [],
  "mood": "neutral",
  "current_delivery_mode": "foot",
  "packages_delivered_this_week": 0,
  "accrued_wages": 0.0,
  "status": "ready"
}
```

---

# 8. Hiring

A new game starts with **zero delivery employees**.

Hiring has an upfront welcome/hiring bonus based on the historical hire number:

| Historical hire number | Fee |
|---:|---:|
| 1 | £50 |
| 2 | £100 |
| 3 | £150 |
| 4 | £200 |
| 5 | £250 |

Pattern: +£50 for each subsequent hire.

This uses a **high-water/history counter**, not current headcount.

Example: if the company has previously hired four employees and one leaves, the next hire is historical hire #5 and costs £250. It does not fall back to the cheap first-hire price.

### Example hiring state

```json
{
  "current_employee_count": 1,
  "total_hires_lifetime": 4,
  "next_hiring_fee": 250.0
}
```

---

# 9. Delivery

The player assigns a **batch/count of packages to a specific employee** rather than manually clicking each package.

Example intention:

> Assign Bob enough eligible packages for this run up to a requested count/capacity.

The backend chooses/validates the actual packages according to canonical eligibility and priority rules.

Exact automatic package priority (for example express first, then earliest due date) is **OPEN**.

## 9.1 Walking delivery duration

For the first vertical slice, a walking delivery cycle lasts **4 game hours**:

- **1 game hour — packing/preparation**
- **3 game hours — out for delivery**

The standard weekday walking schedule therefore supports two cycles:

- Morning cycle: `09:00 → 10:00` packing, `10:00 → 13:00` delivery
- Afternoon cycle: `13:00 → 14:00` packing, `14:00 → 17:00` delivery

During packing, the employee is not available for another assignment. Once packing completes, the employee state becomes `out_for_delivery`. At delivery completion, the assigned packages are completed according to the delivery rules and the employee returns to `ready`.

The backend must track the packing phase separately from the actual delivery phase so that the UI can show the correct operational state.

### Late departure and missed cycles

The `09:00` and `13:00` times are **opportunities**, not forced departures. The player may keep an employee waiting in order to build a fuller load.

A walking cycle still consumes **4 game hours from the moment the assignment/departure cycle begins**:

- 1 game hour packing;
- 3 game hours delivery.

A new cycle may start only if the complete 4-hour cycle can finish by office closing time (`17:00` on weekdays).

Example:

- First cycle starts at `10:00`.
- Packing: `10:00 → 11:00`.
- Delivery: `11:00 → 14:00`.
- Employee becomes `ready` at `14:00`.
- A second walking cycle would finish at `18:00`, after closing.
- Therefore that employee has **missed the second cycle for that day**.

This creates a deliberate player trade-off between waiting for a fuller load and leaving early enough to preserve another delivery opportunity.

Bicycle and car timing can be defined separately later without changing this model.

## 9.2 Capacity per run

| Mode | Capacity |
|---|---:|
| Foot | 10 |
| Bicycle | 20 |
| Car | 50 |

Normal packages consume 1 capacity unit. Express consumes 2.

The player's Logistics trait increases delivery capacity by 10%; exact rounding behaviour is **OPEN** and must be defined before implementation of that modifier.

## 9.3 Runs per working day

| Mode | Local runs/day | Far runs/day |
|---|---:|---:|
| Foot | 2 | 0 |
| Bicycle | 3 | 0 |
| Car | 2 | 1 |

For a car, the intended rule is two local runs **or** one far run for the relevant day allocation. Far delivery behaviour is deferred and should be finalised before implementation.

For the first vertical slice only walking/local delivery is required.

---

# 10. Employee wages and payroll

There is no fixed weekly wage in the current design. Wages accrue based on successfully delivered package count.

| Delivery mode | Employee wage per delivered package |
|---|---:|
| Foot | £2.00 |
| Bicycle | £3.00 |
| Car | £2.50 |

An express package counts as **one package for wages**, despite consuming two delivery-capacity units.

Wages accrue as deliveries occur and are settled every **Tuesday**.

Missed-payroll consequences are **OPEN**.

### Example payroll entry

```json
{
  "employee_id": "emp-0001",
  "packages_delivered": 10,
  "delivery_mode": "foot",
  "rate_per_package": 2.0,
  "amount_due": 20.0
}
```

---

# 11. Finance

Finance must be its own backend domain/module and provide both current balance and an understandable statement/breakdown.

The UI should be able to show at least:

- available cash;
- revenue;
- employee/headcount cost;
- rent;
- loan interest;
- other expenses;
- upcoming obligations.

## 11.1 Starting loan

Initial:

- cash received: **£1,000**
- loan principal: **£1,000**
- interest: **5% every four game weeks**
- initial four-week interest on £1,000: **£50**

Loan repayment mechanics/principal amortisation are **OPEN**. For now, do not assume that paying interest automatically reduces principal.

## 11.2 Scheduled costs

- employee wages: Tuesday;
- office rent: Friday after the first four prepaid weeks;
- loan interest: every four weeks.

## 11.3 Transactions

Every monetary mutation should create a finance transaction rather than silently changing cash.

Recommended categories include:

- `package_revenue`
- `financial_trait_bonus`
- `office_down_payment`
- `rent`
- `rent_late_fee`
- `hiring_bonus`
- `employee_wages`
- `loan_interest`
- future `vehicle_purchase`
- future `upgrade`

### Example finance statement

```json
{
  "cash_balance": 523.0,
  "period": {
    "from": "1980-02-01T00:00:00",
    "to": "1980-02-07T23:59:59"
  },
  "income": {
    "package_revenue": 180.0,
    "trait_bonus": 18.0,
    "total": 198.0
  },
  "expenses": {
    "employee_wages": 36.0,
    "rent": 0.0,
    "loan_interest": 0.0,
    "hiring": 50.0,
    "other": 0.0,
    "total": 86.0
  },
  "net_change": 112.0,
  "liabilities": {
    "loan_principal": 1000.0,
    "accrued_employee_wages": 0.0,
    "next_rent_amount": 50.0,
    "next_interest_estimate": 50.0
  }
}
```

### Example transaction

```json
{
  "id": "txn-000001",
  "game_datetime": "1980-02-01T09:00:00",
  "category": "office_down_payment",
  "amount": -350.0,
  "description": "Small head office contract",
  "reference_id": "office-small-01"
}
```

---

# 12. Vehicles

The architecture should leave room for:

- walking;
- bicycles;
- cars;
- drones;
- future aircraft.

Vehicle purchase prices and operating/fuel costs are **OPEN**.

For MVP Slice 1, no purchased vehicle is necessary because the employee walks.

---

# 13. Persistence

SQLite is the initial persistence technology.

Persistence ownership follows application-inverts-dependency direction:

- The game/application layer owns the persistence interface (the abstraction).
- Infrastructure (the SQLite adapter) implements that interface and depends on the game layer, never the reverse.
- The game/application layer must not import an infrastructure/persistence package.

Avoid generic per-entity repository proliferation; keep the abstraction focused on what the application actually needs.

At minimum, saved state eventually needs to cover:

- game clock/calendar;
- player;
- office contract;
- packages;
- employees;
- finance transactions/balance;
- hiring high-water counter;
- relevant scheduled obligations.

Save/load must preserve game time exactly. No offline catch-up occurs.

---

# 14. API contract

Exact endpoint naming can evolve during implementation, but this is the intended shape.

The API exposes authoritative backend state and accepts requested actions. It does not expose the internal rule engine as a separate requirement for MVP. Every mutating action must be validated by backend rules before any state transition is persisted.

Suggested initial resources:

```text
GET  /api/v1/game
GET  /api/v1/clock
POST /api/v1/clock/speed
POST /api/v1/clock/pause
POST /api/v1/clock/skip-to-next-opening

GET  /api/v1/player

GET  /api/v1/offices
POST /api/v1/offices/select
GET  /api/v1/office

GET  /api/v1/packages

GET  /api/v1/employees
POST /api/v1/employees/hire

POST /api/v1/deliveries/assign

GET  /api/v1/finance
GET  /api/v1/finance/transactions
```

Not all endpoints are required in the first implementation milestone.

## 14.1 Standard error shape

Use a predictable machine-readable error object.

```json
{
  "error": {
    "code": "INSUFFICIENT_DELIVERY_CAPACITY",
    "message": "Employee does not have enough capacity for this assignment.",
    "details": {
      "employee_id": "emp-0001",
      "available_capacity": 10,
      "requested_capacity": 12
    }
  }
}
```

Do not make the frontend parse human-readable strings to understand failures.

Invalid rule checks or state transitions should return machine-readable errors and leave authoritative state unchanged.

## 14.2 Example game-state response

`GET /api/v1/game` returns the state under a single `game-state` wrapper (this is the contract the frontend consumes):

```json
{
  "game-state": {
    "game": {
      "status": "running",
      "game_datetime": "1980-02-01T09:00:00",
      "speed": 1
    },
    "player": {
      "id": "player-1",
      "cash": 600.0,
      "trait": "financial"
    },
    "office": null,
    "operations": {
      "stored_packages": 18,
      "out_for_delivery": 0,
      "delivered_today": 0
    },
    "finance": {
      "accrued_wages": 0.0,
      "next_rent": 50.0,
      "loan_principal": 1000.0
    }
  }
}
```

`office` is `null` until an office has been selected; once selected it carries:

```json
{
  "id": "office-small-01",
  "storage_used": 18,
  "storage_capacity": 100,
  "employee_count": 1,
  "employee_capacity": 5
}
```

`GET /api/v1/game` is read-only and must not create or seed state as a side effect.

## 14.3 Example clock response

`GET /api/v1/clock` returns the clock under a single `clock` wrapper:

```json
{
  "clock": {
    "game_datetime": "1980-02-01T09:00:00",
    "day_of_week": "friday",
    "speed": 1,
    "paused": false,
    "office_open": true,
    "days_until_next_payroll": 4,
    "days_until_next_rent": 0
  }
}
```

## 14.4 Clock mutation endpoints

The clock supports three mutations in addition to `GET /api/v1/clock`. All are deterministic and use only fictional game calendar time (never the host wall clock). The backend owns the authoritative clock; these endpoints mutate it and return the updated state.

### Request bodies

| Endpoint | Body | Field | Type | Allowed values |
|---|---|---|---:|---|
| `POST /api/v1/clock/speed` | `{"speed": 1}` | `speed` | integer | `1`, `2`, `3` |
| `POST /api/v1/clock/pause` | `{"paused": true}` | `paused` | boolean | `true`, `false` |
| `POST /api/v1/clock/skip-to-next-opening` | `{}` (or empty) | — | — | no fields allowed |

- **Speed** changes only the configured speed. It is allowed while paused; the clock stays paused until explicitly unpaused. Advancement rates remain those defined in §2.2: speed 1 → 120, speed 2 → 240, speed 3 → 360 game seconds per real second.
- **Pause** changes only the `paused` flag (`true` pauses advancement, `false` resumes). It does not change speed or game time; while paused, deterministic advancement makes no change to game time.
- **Skip-to-next-opening** accepts an empty body `{}` (a zero-length body is also accepted) and contains no fields. It changes only game date/time and preserves both the current speed and the `paused` state.

### Skip-to-next-opening semantics

Working schedule (§2.4): Mon–Fri 09:00–17:00, Sat 10:00–13:00, Sun closed. Opening intervals are half-open `[opening, closing)`: exactly at opening → open; exactly at closing → closed.

- **Currently open** → safe no-op: game time unchanged, speed and `paused` preserved, HTTP 200 with the current clock snapshot. It does NOT advance to the next working day merely because it was called while already open (this avoids accidental loss of a playable working period).
- **Currently closed** → advance to the earliest upcoming opening instant. Examples: Mon 08:00 → Mon 09:00; Mon 17:00 → Tue 09:00; Fri 17:00 → Sat 10:00; Sat 09:00 → Sat 10:00; Sat 13:00 → Mon 09:00; any Sunday → Mon 09:00.

M1C has no packages/events/scheduled simulation, so skip may move clock time directly. When time-dependent simulation is later introduced, skipped intervals must be processed through appropriate simulation semantics rather than bypassing game effects (not implemented in M1C).

### Success responses

All three mutation endpoints return HTTP `200 OK`, `Content-Type: application/json`, with the complete **updated** canonical clock snapshot under the same `clock` wrapper as `GET /api/v1/clock`. No separate mutation-response shapes are introduced.

### Validation policy

- Speed and pause: body must be valid JSON; the required field must be present with the correct type; unknown fields are rejected; multiple/trailing JSON values are rejected.
- Skip: a zero-length body is accepted, `{}` is accepted, any JSON fields are rejected, malformed JSON is rejected, trailing JSON values are rejected.
- Rejected input never mutates authoritative state.

### Clock error codes

All clock API errors use the canonical envelope from §14.1 (`error.code`, `error.message`, optional `error.details`). Plain-text errors are not used on the clock API surface.

| Code | HTTP | Meaning |
|---|---:|---|
| `INVALID_JSON` | 400 | malformed JSON; more than one JSON value; body cannot be decoded as JSON where required |
| `INVALID_REQUEST` | 400 | missing field; wrong type; unknown field; skip request contains fields; shape otherwise invalid (`details` may name the field) |
| `INVALID_CLOCK_SPEED` | 400 | `speed` is an integer but not one of `1`, `2`, `3`; previous speed unchanged (`details` may carry `{"allowed":[1,2,3]}`) |
| `METHOD_NOT_ALLOWED` | 405 | unsupported HTTP method on a clock route |

---

# 15. Deferred scope

Explicitly **not required for the first prototype**:

- weather;
- external/random event system;
- drones;
- aircraft;
- far/international delivery implementation;
- employee training;
- retirement;
- detailed mood/resignation system;
- decorations;
- warehouse equipment upgrades;
- automatic delivery optimisation;
- WebSockets;
- multiplayer;
- online/offline cloud service;
- anti-cheat/tamper protection beyond using an internal game clock.

The future event system may supply temporary rule modifiers, but events must still pass through normal rule validation and legal state transitions.

These should be addable later rather than designed into every first-pass function.

---

# 16. Open decisions

The following are intentionally unresolved:

1. Small/medium package generation probability.
2. Large package normal and express prices.
3. Exact behaviour when storage is full.
4. Package auto-selection priority for an assigned batch.
5. Numeric Snail/Chicken/Cheetah speed modifiers.
6. Logistics-trait capacity rounding.
7. Vehicle purchase/fuel/maintenance costs.
8. Missed payroll consequences.
9. Loan principal repayment/amortisation.
10. Exact grace/recovery flow after a missed rent payment.
11. Detailed far-delivery scheduling.
12. Exact package-generation tick boundary semantics; the canonical rate remains 2 packages per open working hour.

Do not allow a coding agent to silently decide these permanently. Temporary implementation assumptions should be clearly labelled and isolated in configuration.

---

# 17. First vertical slice acceptance target

The first playable backend/UI handover does **not** need the whole specification.

It succeeds when:

1. a new game can start at 1 February 1980;
2. the player has £1,000 cash/loan;
3. a small office can be selected and its £350 down payment recorded;
4. local normal small/medium packages can exist in storage;
5. one walking employee can be hired;
6. the employee can be assigned an eligible batch;
7. walking capacity of 10 is enforced;
8. delivery can complete;
9. package revenue is recorded;
10. £2 per delivered package employee wage accrues;
11. the resulting state can be retrieved as JSON;
12. state can be persisted/reloaded;
13. the React frontend can consume the API without implementing backend game rules.

That is the first end-to-end proof that the architecture works.
