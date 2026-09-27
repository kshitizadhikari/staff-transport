# Staff Transport Management System

A transport operations platform for organizations that move staff between homes, offices, event venues, and other destinations.

The system is designed around **manager-driven dispatch**, not public ride booking.

## Product Goal

Give one manager a single place to:

- manage staff, drivers, and vehicles
- create and schedule trips
- assign drivers and passengers
- coordinate recurring staff transportation
- manage event transportation
- monitor trip execution
- track pickup/no-show status
- review transport history and operational reports

Give drivers a focused mobile workflow to execute assigned trips.

Give staff a simple interface to see their transportation schedule and trip status.

## Roles

### Manager / Admin

Full operational control.

- Manage staff
- Manage drivers
- Manage vehicles
- Create/edit/cancel trips
- Assign drivers and vehicles
- Assign passengers
- Create recurring schedules
- Create events
- Monitor active trips
- View driver and trip status
- Review reports and audit history

### Driver

Mobile-first execution workflow.

- View today's assigned trips
- View ordered stops and passengers
- Open navigation
- Start a trip
- Mark passenger picked up
- Mark passenger no-show
- Complete a stop/trip
- Receive trip changes and notifications
- Send periodic location updates while on an active trip

### Staff

Passenger-facing workflow.

- View upcoming transportation
- View assigned driver/vehicle
- View pickup location and time
- View trip/event details
- Receive transport notifications
- See current trip status

## Recommended Stack

### Web

- Next.js
- TypeScript
- Tailwind CSS
- shadcn/ui
- TanStack Query
- React Hook Form
- Zod

The web app contains the manager dashboard and staff experience.

### Mobile

- React Native
- Expo

The driver application is mobile-first because background GPS, push notifications, navigation, and unreliable-network handling are central requirements.

### Backend

- Go
- Gin
- GORM
- Asynq

The backend is a modular monolith. Do not split it into microservices unless there is a demonstrated operational need.

### Data

- PostgreSQL
- PostGIS
- Redis

PostgreSQL is the system of record. PostGIS is used for geographic data and queries. Redis is used for caching, transient state, rate limiting where needed, and background jobs through Asynq.

### Infrastructure

- Docker / Docker Compose for local development
- AWS for deployment
- Amazon ECR for container images
- AWS ECS/Fargate or EC2 for runtime
- Bitbucket for source control
- Azure DevOps for CI/CD orchestration

### External Services

- Google Maps Platform for geocoding, directions, and map/navigation features
- Push notification provider compatible with Expo / FCM as implementation requires
- S3-compatible object storage for future driver/vehicle documents

## Core Domain Model

The most important domain object is a **Trip**.

A trip represents one transportation operation and contains:

- scheduled date/time
- trip type
- driver
- vehicle
- ordered stops
- passengers
- execution status
- timestamps
- optional event association

Examples:

```text
Morning Staff Pickup
Staff Homes -> Office

Office -> Event Venue

Event Venue -> Staff Homes

Office -> Staff Home

Custom Transport Trip
```

Do not model the system as a simple `driver -> staff` relationship. A single trip can have multiple passengers and multiple stops.

## Suggested Repository Structure

```text
staff-transport/
├── apps/
│   ├── web/                 # Next.js manager + staff application
│   └── driver-mobile/       # React Native + Expo driver app
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   └── worker/
│   ├── internal/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── staff/
│   │   ├── drivers/
│   │   ├── vehicles/
│   │   ├── trips/
│   │   ├── events/
│   │   ├── dispatch/
│   │   ├── locations/
│   │   └── notifications/
│   ├── migrations/
│   └── pkg/
├── docs/
├── infra/
├── .env.example
├── .gitignore
├── AGENTS.md
└── README.md
```

## MVP

### Manager

- Authentication
- Staff CRUD
- Driver CRUD
- Vehicle CRUD
- Trip CRUD
- Passenger assignment
- Driver/vehicle assignment
- Today's operational dashboard
- Trip status tracking
- Basic recurring schedules
- Basic event transport setup

### Driver

- Authentication
- Today's trips
- Trip detail
- Ordered stops
- Passenger list
- Start trip
- Mark picked up
- Mark no-show
- Complete trip
- Navigation deep link
- Basic location updates while trip is active

### Staff

- Authentication
- Upcoming trips
- Today's trip
- Driver/vehicle information
- Pickup information
- Basic notifications/status

## Post-MVP

Build in this order unless new operational evidence changes the priority:

1. Push notifications
2. Better recurring schedules
3. Live driver tracking
4. Event transportation workflow
5. Route optimization
6. Reporting and analytics
7. Vehicle maintenance and fuel records
8. Offline driver support
9. Advanced permissions and audit tooling

## Non-Goals for V1

Do not build these unless explicitly requested:

- public ride-hailing
- passenger bidding
- payment processing
- driver marketplace
- microservice architecture
- Kubernetes
- Kafka/event streaming infrastructure
- AI-based dispatching
- custom routing engine

## Product Principles

1. **Manager plans, drivers execute, staff consume information.**
2. **Trips are the operational unit.**
3. **Every passenger state change should be auditable.**
4. **Location is sensitive operational data and should be minimized and protected.**
5. **Driver workflows must remain usable with poor connectivity.**
6. **Operational correctness matters more than visual complexity.**
7. **Prefer simple, explicit backend modules over premature abstraction.**

## Getting Started

Project implementation should start by creating the monorepo skeleton, local PostgreSQL/PostGIS and Redis services, environment configuration, backend health endpoint, and web application shell.

See:

- [Runbook](./RUNBOOK.md)
- [AGENTS.md](./AGENTS.md)
- [Product Requirements](./docs/PRODUCT_REQUIREMENTS.md)
- [Architecture](./docs/ARCHITECTURE.md)
- [Database Design](./docs/DATABASE.md)
- [API Conventions](./docs/API_CONVENTIONS.md)
- [Development Guide](./docs/DEVELOPMENT.md)
- [Decisions](./docs/DECISIONS.md)

## Status

Planning / initial implementation.

Implemented so far:

- monorepo skeleton, local PostgreSQL/PostGIS + Redis, environment config
- backend health/readiness endpoints, GORM + Redis wiring
- authentication API: `POST /api/v1/auth/login`, `/refresh`, `/logout`, `GET /api/v1/me`
- local seeding via `make seed` (manager, ten staff, ten drivers, ten vehicles)
- web login screen, authenticated home, and CORS support for the web origin
- web app light/dark theme toggle (persisted, defaults to the system preference)
- manager directory & fleet API (manager-only):
  - `GET/POST /api/v1/staff`, `GET/PATCH/DELETE /api/v1/staff/:id`
  - `GET/POST /api/v1/drivers`, `GET/PATCH/DELETE /api/v1/drivers/:id`
  - `GET/POST /api/v1/vehicles`, `GET/PATCH/DELETE /api/v1/vehicles/:id`
  - pagination, search, status filters; create provisions the linked user account
- manager web UI for staff, drivers, and vehicles: list with search/status
  filters, create/edit forms, and deactivate with confirmation
- manager trip UI: trips list with status/date filters, trip creation with
  ordered stops and passenger assignment, and trip detail with edit and cancel
- trip management API (manager-only):
  - `POST/GET /api/v1/trips`, `GET/PATCH /api/v1/trips/:id`, `POST /api/v1/trips/:id/cancel`
  - ordered stops, passenger assignment, driver/vehicle assignment
  - capacity and assignment validation; organization-timezone date filtering
  - audit records for creation, updates, cancellation, and assignment changes
- driver trip execution API (assigned driver only; transitions are idempotent):
  - `GET /api/v1/me/trips`, `GET /api/v1/me/trips/:id` (driver's assigned trips,
    or staff's own transportation)
  - `POST /api/v1/trips/:id/start`, `POST /api/v1/trips/:id/complete`
  - `POST /api/v1/trips/:id/stops/:stopId/arrive|depart`
  - `POST /api/v1/trips/:id/passengers/:passengerId/pickup|no-show`
  - audit records for start, completion, pickup, and no-show
- driver mobile app (Expo, driver accounts only):
  - sign in, today's assigned trips, and trip detail with ordered stops
  - start/complete trip, stop arrive/depart, and passenger pickup/no-show
- driver location ingestion: `POST /api/v1/trips/:id/locations` accepts a batch
  of samples for an active (or just-completed) trip, assigned driver only;
  telemetry is append-only, deduplicated by device timestamp, and tolerant of
  delayed or out-of-order uploads
- notifications: device push tokens (`POST /api/v1/me/push-tokens`) and an in-app
  feed (`GET /api/v1/me/notifications`); trip assignment and cancellation enqueue
  Asynq jobs delivered via an Expo push provider in the worker (migration
  `0002_push_tokens`)
- recurring schedules (`/api/v1/recurring-schedules`, manager-only): weekly
  definition with a trip template, and idempotent generation of concrete trips
  for a date range (unique `(schedule, occurrence_date)`); generated trips are
  ordinary, independently editable trips
- event transport (`/api/v1/events`, manager-only): event details (name, venue,
  coordinates, start/end) and participants; trips serving an event are those
  with that `event_id`, listed on the event. Deleting an event preserves and
  unlinks its trips.

Known limitations:

- web and mobile sessions live in memory, so a reload/restart requires signing in
  again
- trip stops cannot be replaced after creation (cancel and recreate); driver and
  vehicle double-booking detection and geocoded stop coordinates are not
  implemented yet
- driver-mobile location capture and push-token registration are not implemented
  yet (the backend endpoints exist)
- recurring generation is exposed via an API endpoint; a periodic Asynq
  scheduler that invokes it automatically is not wired yet
- the `event_trips` table is reserved for future use; event/trip association is
  currently `trips.event_id`
- persistent sessions (httpOnly refresh cookie) and the driver mobile auth flow
  are not implemented yet

Next: manager events UI, then the recurring-schedule scheduler and mobile capture.
