# AGENTS.md

Primary instructions for coding agents working in this repository.

## 1. Project Context

This repository contains a staff transportation management platform with three roles:

* `manager`: plans and operates transportation
* `driver`: executes assigned trips through the mobile app
* `staff`: views assigned transportation and receives updates

This is an internal staff transportation system, not a public ride-hailing marketplace.

---

## 2. Source of Truth

Use these documents as the authoritative project references:

1. `README.md`
2. `docs/PRODUCT_REQUIREMENTS.md`
3. `docs/ARCHITECTURE.md`
4. `docs/DATABASE.md`
5. `docs/API_CONVENTIONS.md`
6. `docs/DEVELOPMENT.md`
7. `docs/DECISIONS.md`

Read the documents relevant to the task before making implementation decisions. For architectural, product, or cross-cutting changes, inspect all relevant source-of-truth documents.

When documented product behavior conflicts with an implementation preference, preserve the documented behavior and explicitly flag the conflict.

---

## 3. Technology Rules

### Web

Use:

* Next.js
* TypeScript
* Tailwind CSS
* shadcn/ui
* TanStack Query
* React Hook Form
* Zod

The web application is shared by manager and staff users.

UI visibility is not authorization. All protected operations must be authorized on the server.

### Driver Mobile

Use React Native with Expo.

Assume:

* intermittent connectivity
* denied location permissions
* battery/background restrictions
* delayed requests
* failed requests

The driver workflow must remain usable under these conditions.

### Backend

Use:

* Go
* Gin
* GORM
* PostgreSQL
* PostGIS
* Redis
* Asynq

Use a modular monolith.

Do not introduce microservices unless there is a concrete documented requirement, ADR, or issue supporting the change.

---

## 4. Domain Rules

### Trips

Trip is the primary operational entity.

Use one trip model with:

* trip type
* ordered stops
* passengers
* driver/vehicle assignments
* execution state

Do not create separate domain implementations for scenarios such as morning pickup, event transport, and home drop.

Stop order must be explicit and persisted.

### Passengers

Passengers belong to trips through a trip-passenger association.

Do not represent trip passengers using a single `staff_id` directly on the trip.

### Trip States

```text
scheduled
assigned
in_progress
completed
cancelled
```

### Passenger States

```text
assigned
picked_up
no_show
completed
cancelled
```

Passenger execution state must remain separate from overall trip state.

### Audit

Audit important state changes, including:

* trip creation, update, and cancellation
* driver assignment changes
* vehicle assignment changes
* passenger assignment/removal
* passenger pickup/no-show status
* trip start/completion
* relevant role or permission changes

---

## 5. Security Rules

Never trust client-supplied:

* role IDs
* owner IDs
* driver IDs
* passenger IDs
* other authorization-sensitive identifiers

Every protected mutation must authorize the acting user on the server.

Protect:

* passwords and credentials
* refresh tokens
* home addresses
* precise staff locations
* precise driver locations
* driver-license information
* vehicle documents

Never log passwords or tokens.

Do not log unnecessary precise location data.

Validate request bodies at API boundaries.

Use parameterized queries or ORM parameter binding. Never construct SQL from untrusted input.

Client-side authorization and validation are not security controls.

---

## 6. API Rules

Follow `docs/API_CONVENTIONS.md`.

APIs should use:

* resource-oriented endpoints
* consistent error responses
* pagination for collections
* explicit authorization
* request/response DTOs

Do not expose database entities directly as API contracts.

Keep business rules and authoritative state in backend services where practical.

---

## 7. Database Rules

PostgreSQL is the source of truth.

* All schema changes require migrations.
* Never manually mutate production schema.
* Use transactions when a business operation updates multiple related records and partial completion could create invalid state.
* Use PostGIS for geographic data.
* Store a human-readable address and coordinates when a location is geocoded.
* Preserve historical trip-stop addresses as snapshots when historical accuracy matters.
* Never assume a staff member's current home address is the historical pickup address for an old trip.

---

## 8. Location Tracking

Location tracking must have an operational purpose.

For the MVP:

* enable location updates during an active trip
* use coarse/periodic updates rather than excessive frequency
* handle denied permissions cleanly
* tolerate delayed and out-of-order updates
* record server timestamps

Treat location events as append-only or immutable telemetry where practical.

Derive current state from telemetry rather than rewriting historical events.

---

## 9. Frontend Rules

Backend APIs remain authoritative for business rules, permissions, and state.

Client-side validation exists for UX and does not replace server validation.

### Manager UI

Prioritize operational clarity:

* filters
* search
* statuses
* compact tables
* clear actions
* loading states
* empty states
* actionable error states
* confirmation for destructive operations

### Driver UI

Prioritize:

* large touch targets
* minimal steps
* clear current stop
* obvious trip state
* navigation action
* retry behavior

Do not overload the driver workflow with manager functionality.

---

## 10. Error Handling and Observability

Never silently swallow:

* API errors
* database errors
* background-job errors
* external-service failures

User-facing errors should be actionable without exposing internal implementation details.

Backend logs should be structured and contain sufficient context for diagnosis.

---

## 11. Testing Rules

New backend business logic should have unit tests.

New API behavior should have integration tests where practical.

Important workflows should progressively receive end-to-end coverage, especially:

1. manager creates trip
2. manager assigns driver, vehicle, and passengers
3. driver starts trip
4. driver records passenger outcome
5. driver completes trip
6. staff sees updated status

Do not write tests that merely restate framework behavior.

---

## 12. Background Jobs

Use Asynq for:

* scheduled notifications
* recurring-trip generation
* retryable external-service work
* other non-blocking background work

Background jobs must be safe to retry.

Use idempotency keys or unique business identifiers when duplicate execution could create duplicate trips, notifications, or other side effects.

---

## 13. External Services

Wrap external providers behind interfaces or dedicated modules.

Do not spread provider-specific calls throughout domain code.

Examples:

```text
internal/maps/
internal/notifications/
```

This keeps external-provider logic replaceable and testable.

---

## 14. Time and Scheduling

Transportation scheduling requires explicit timezone handling.

* Store timestamps in UTC at the database/API boundary where appropriate.
* Represent the organization's configured timezone explicitly.
* Use the configured timezone for recurring schedules and user-facing times.
* Never implicitly rely on the server's local timezone.

---

## 15. Change Discipline

Before modifying an existing module:

1. inspect related handlers, services, and repositories
2. inspect relevant tests
3. inspect relevant database migrations
4. check API compatibility
5. identify affected domain behavior
6. make the smallest coherent change
7. add or update tests
8. run relevant validation

Do not perform broad refactors while implementing an unrelated feature.

---

## 16. Agent Workflow

### Step A — Understand

Identify:

* affected domain entities
* affected user roles
* API endpoints
* UI screens
* database changes
* external services
* authorization requirements

Read only the relevant source-of-truth documents and code needed to understand the task.

### Step B — Plan

Before editing multiple files, state a concise implementation plan.

Keep the plan proportional to the task.

### Step C — Implement

Prefer backend and data changes before depending on them in the UI, unless the task specifically requires the opposite.

Keep changes focused and minimal.

### Step D — Validate

Run relevant:

* formatters
* unit tests
* integration tests
* type checks
* linters
* builds

Do not run unrelated expensive checks unless required.

### Step E — Review

Before considering the task complete, check applicable:

* authorization
* validation
* duplicate execution
* race conditions
* transaction boundaries
* timezone handling
* failure paths
* historical-data correctness
* API compatibility

---

## 17. Definition of Done

A feature is not complete merely because it compiles.

For applicable changes, verify:

* authorization exists
* validation exists
* error handling exists
* loading/empty/error UI states exist
* required database migration exists
* tests exist
* API contract is documented
* required audit behavior exists
* logging/observability is sufficient
* local development setup still works

Do not add unnecessary ceremony when a requirement is not applicable to the change.

---

## 18. What Not to Do

Do not:

* introduce microservices prematurely
* duplicate business rules between frontend and backend
* store passwords in plaintext
* trust client authorization claims without server verification
* silently ignore failed jobs or notifications
* overwrite historical trip-stop addresses without preserving required history
* build a custom map/routing engine
* add technology without a concrete requirement
* modify unrelated files
* perform broad refactors for unrelated tasks
* introduce dependencies without justification
* change established architecture without checking the project documentation

---

## 19. Agent Efficiency

Optimize for focused context and minimal unnecessary work.

* Read only relevant files after understanding the task.
* Prefer targeted searches over reading entire directories.
* Avoid dumping entire files into responses.
* Avoid repeating information already available in the repository.
* Keep generated explanations concise unless detailed explanation is requested.
* Make the smallest coherent diff.
* Do not modify unrelated files.
* Run only validation relevant to the changed components.
* Avoid unnecessary tool calls, builds, and repeated tests.
* Stop when the requested task is implemented and relevant validation passes.
