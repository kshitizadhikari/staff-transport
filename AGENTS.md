# AGENTS.md

This file is the primary instruction set for coding agents working in this repository.

## 1. Project Context

This repository contains a staff transportation management platform.

There are three roles:

- `manager`: plans and operates transportation
- `driver`: executes assigned trips from a mobile application
- `staff`: views assigned transportation and receives updates

The system is not a public ride-hailing marketplace.

## 2. Source of Truth

Before making implementation decisions, read these files in order:

1. `README.md`
2. `docs/PRODUCT_REQUIREMENTS.md`
3. `docs/ARCHITECTURE.md`
4. `docs/DATABASE.md`
5. `docs/API_CONVENTIONS.md`
6. `docs/DEVELOPMENT.md`
7. `docs/DECISIONS.md`

When product requirements and implementation preferences conflict, preserve the documented product behavior and point out the conflict.

## 3. Technology Rules

### Web

Use:

- Next.js
- TypeScript
- Tailwind CSS
- shadcn/ui
- TanStack Query
- React Hook Form
- Zod

The web app is shared by manager and staff users. Hide features through authorization; do not rely on UI hiding alone for security.

### Driver Mobile

Use React Native with Expo.

Driver functionality should assume:

- intermittent connectivity
- location permission can be denied
- battery/background restrictions exist
- network requests can fail or be delayed

### Backend

Use Go with Gin, GORM, PostgreSQL/PostGIS, Redis, and Asynq.

Keep the backend a modular monolith.

Do not introduce microservices without a concrete requirement documented in an ADR or issue.

## 4. Domain Rules

### Trip is the main operational entity

Do not create separate domain implementations for every transport scenario such as `morning_pickup_service`, `event_transport_service`, and `home_drop_service`.

Use one trip model with a type and related stops/passengers.

### Stops are ordered

A trip can contain multiple stops. Stop order must be explicit and persisted.

### Passengers belong to trips through an association

Never store a single `staff_id` directly on a trip when representing passengers.

Use a trip-passenger relationship.

### Execution state must be explicit

At minimum support:

```text
scheduled
assigned
in_progress
completed
cancelled
```

Passenger execution state should be separate from overall trip state.

At minimum support:

```text
assigned
picked_up
no_show
completed
cancelled
```

### Audit important state changes

At minimum audit:

- trip creation/update/cancellation
- driver assignment changes
- vehicle assignment changes
- passenger assignment/removal
- passenger pickup/no-show status
- trip start/completion
- relevant permission or role changes

## 5. Security Rules

Never trust client-supplied role, owner, driver, or passenger IDs.

Every protected mutation must authorize the acting user on the server.

Protect:

- passwords/credentials
- refresh tokens
- home addresses
- precise staff locations
- precise driver locations
- driver license information
- vehicle documents

Do not log tokens, passwords, or unnecessary precise location data.

Validate request bodies at the API boundary.

Use parameterized queries / ORM parameter binding. Never construct SQL from raw untrusted strings.

## 6. API Rules

Follow `docs/API_CONVENTIONS.md`.

Use resource-oriented endpoints, consistent error responses, pagination for collections, and explicit authorization.

Do not expose database entities directly as API contracts. Define request/response DTOs.

## 7. Database Rules

PostgreSQL is the source of truth.

Use migrations for schema changes.

Do not manually mutate production schema.

Use transactions when a business operation updates multiple related records and partial completion would create invalid operational state.

Use PostGIS for geographic data rather than storing only string addresses.

Store both:

- human-readable address
- coordinates when the location is geocoded

Keep address snapshots on trip stops when historical accuracy matters. Do not assume a staff member's current home address is the historical pickup location for an old trip.

## 8. Location Tracking Rules

Location tracking is only necessary for operational use cases.

Do not continuously track a driver when there is no operational reason.

For MVP:

- enable location updates during an active trip
- send coarse/periodic updates rather than excessive frequency
- handle missing permissions cleanly
- tolerate delayed/out-of-order updates
- record a server timestamp

Treat location events as append-only or immutable telemetry where practical; derive current state from them rather than rewriting historical events.

## 9. Frontend Rules

Keep business rules in shared backend APIs where possible.

Client-side validation improves UX but is not security.

Prefer server-side authorization and authoritative state.

For manager screens, favor operational clarity:

- filters
- statuses
- search
- compact tables
- clear actions
- error states
- empty states
- confirmation for destructive operations

For the driver app, favor:

- large touch targets
- minimal steps
- clear current stop
- obvious trip state
- navigation action
- retry behavior

Do not overload the driver UI with manager functionality.

## 10. Error Handling

Errors should be actionable.

Never swallow API, database, or background-job errors silently.

User-facing messages should explain what failed without leaking internal details.

Backend logs should contain structured context useful for diagnosis.

## 11. Testing Rules

New backend business logic should have unit tests.

New API behavior should have integration tests where practical.

Important workflows must have end-to-end coverage over time, especially:

1. manager creates trip
2. manager assigns driver/vehicle/passengers
3. driver starts trip
4. driver records passenger outcome
5. driver completes trip
6. staff sees updated status

Do not write tests that only restate framework behavior.

## 12. Background Jobs

Use Asynq for:

- scheduled notifications
- recurring-trip generation
- retryable external-service work
- other non-blocking background tasks

Background jobs must be safe to retry.

Prefer idempotency keys or unique business identifiers where duplicate execution could create duplicate trips or notifications.

## 13. External APIs

Wrap external providers behind interfaces/modules.

Do not spread Google Maps or notification-provider calls throughout domain code.

Example:

```text
internal/maps/
internal/notifications/
```

This makes provider replacement and testing easier.

## 14. Change Discipline

Before changing an existing module:

1. inspect related handlers/services/repositories
2. inspect existing tests
3. inspect database migrations
4. identify API compatibility impact
5. make the smallest coherent change
6. add/update tests
7. run relevant checks

Do not perform broad refactors while implementing an unrelated feature.

## 15. Definition of Done

A feature is not complete when the happy-path code compiles.

Consider it complete only when applicable:

- authorization exists
- validation exists
- error handling exists
- loading/empty/error UI states exist
- database migration exists
- tests exist
- API contract is documented
- audit behavior is implemented
- observability/logging is sufficient
- local setup still works

## 16. Agent Workflow

For each feature:

### Step A: Understand

Identify affected domain entities, user role, API endpoints, UI screens, and data changes.

### Step B: Plan

State a small implementation plan before editing multiple files.

### Step C: Implement

Implement backend/data changes before depending on them in the UI unless the task specifically requires the opposite.

### Step D: Validate

Run formatting, tests, type checks, linting, and build checks relevant to the changed components.

### Step E: Review

Check authorization, duplicate execution, race conditions, timezone handling, failure paths, and historical-data correctness.

## 17. Time and Scheduling

The business involves scheduled transportation, so timezone handling is important.

Store timestamps in UTC at the database/API boundary where appropriate.

Represent the organization's configured timezone explicitly for recurring schedules and user-facing times.

Never rely on the server's local timezone implicitly.

## 18. What Not to Do

Do not:

- introduce microservices prematurely
- duplicate business rules in frontend and backend
- store passwords in plain text
- trust client authorization claims without server verification
- silently ignore failed notifications/jobs
- overwrite historical trip stop addresses without preserving history
- build a custom map/routing engine
- add technology because it sounds scalable rather than because the current workload needs it
