# Architecture

## 1. Architectural Style

Use a **modular monolith** for the backend and a monorepo for applications.

```text
                 ┌─────────────────────────────┐
                 │          Next.js Web        │
                 │  Manager + Staff interfaces │
                 └──────────────┬──────────────┘
                                │ HTTPS / JSON
                                ▼
                 ┌─────────────────────────────┐
                 │            Go API            │
                 │       Modular Monolith       │
                 ├─────────────────────────────┤
                 │ Auth                         │
                 │ Staff                        │
                 │ Drivers                      │
                 │ Vehicles                     │
                 │ Trips                        │
                 │ Events                       │
                 │ Dispatch                     │
                 │ Locations                    │
                 │ Maps                         │
                 │ Notifications                │
                 └──────────────┬──────────────┘
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
          ┌──────────────────┐     ┌────────────────┐
          │ PostgreSQL       │     │ Redis          │
          │ + PostGIS        │     │ + Asynq        │
          └──────────────────┘     └───────┬────────┘
                                           ▼
                                    Background Worker

                 ┌─────────────────────────────┐
                 │ React Native + Expo         │
                 │ Driver application          │
                 └──────────────┬──────────────┘
                                │ HTTPS / location
                                └──────────────► Go API
```

## 2. Why a Modular Monolith

The system is transaction-heavy around a small number of tightly related workflows:

- trip assignment
- passenger assignment
- stop execution
- notifications
- location updates

Keeping these inside one deployable backend makes transactions, debugging, development, and deployment simpler.

## 3. Backend Module Boundaries

Each module should own its business behavior rather than exposing persistence directly.

```text
internal/
├── auth/
├── users/
├── staff/
├── drivers/
├── vehicles/
├── trips/
├── events/
├── dispatch/
├── locations/
├── maps/
└── notifications/
```

A typical module should separate:

```text
handler -> service -> repository -> database
```

Not every small helper needs its own abstraction. Avoid interfaces added only to satisfy a pattern.

## 4. Transaction Boundaries

Examples of operations that should be transactional where appropriate:

### Assign trip

```text
validate trip
validate driver availability
validate vehicle availability/capacity
assign driver
assign vehicle
assign passengers
write audit record
```

### Complete trip

```text
validate caller
validate current trip state
update trip
update relevant passenger states
record completion timestamp
write audit record
enqueue follow-up notification
```

The notification should not determine whether the transaction commits successfully.

## 5. Background Worker

Use Asynq for work that should not block API requests.

Examples:

- scheduled notifications
- recurring trip generation
- retryable external API calls
- cleanup tasks

Jobs must be retry-safe.

## 6. Realtime

Start with ordinary REST APIs for CRUD and state changes.

For live operational updates, introduce SSE or WebSocket when the use case actually requires it.

Do not build a realtime event bus before there is a realtime feature.

## 7. Maps

Encapsulate maps functionality behind a provider module.

```text
internal/maps/
    Geocode()
    Route()
    DistanceMatrix()
```

The exact provider can change without changing trip-domain logic.

## 8. Location Data

Driver telemetry should be modeled separately from business entities.

Conceptually:

```text
driver_locations
- id
- driver_id
- trip_id
- point
- recorded_at
- received_at
```

The system should distinguish between:

- when the driver device says the location occurred
- when the server received it

This matters for delayed mobile uploads.

## 9. Frontend Data Flow

Use TanStack Query for server state.

Avoid making global client state the source of truth for trips.

Recommended flow:

```text
UI
 ↓
Query / Mutation
 ↓
API client
 ↓
Go API
 ↓
PostgreSQL
```

Invalidate/refetch relevant queries after mutations or use precise cache updates.

## 10. Mobile Network Strategy

The driver app should eventually support a small local queue for actions such as:

```text
pickup passenger
mark no-show
complete stop
```

Those actions should contain an idempotency key so retries do not create duplicate state transitions.

The initial MVP may start with online execution and robust retry behavior before implementing full offline synchronization.

## 11. Deployment

Initial target:

```text
Bitbucket
   ↓
Azure DevOps Pipeline
   ↓
Docker build
   ↓
Amazon ECR
   ↓
AWS runtime
```

Keep application containers stateless. State belongs in PostgreSQL, Redis, and object storage.

## 12. Observability

At minimum:

- structured application logs
- request correlation/request ID
- job IDs for background work
- error logging with safe context
- health endpoint
- readiness endpoint

Later:

- OpenTelemetry
- metrics dashboards
- distributed traces
- centralized log aggregation
