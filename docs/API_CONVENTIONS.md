# API Conventions

Base path:

```text
/api/v1
```

## Authentication

Example:

```http
Authorization: Bearer <access-token>
```

Refresh-token implementation details should remain isolated from business modules.

## Resources

Use resource-oriented endpoints.

Examples:

```text
GET    /api/v1/staff
POST   /api/v1/staff
GET    /api/v1/staff/:id
PATCH  /api/v1/staff/:id
DELETE /api/v1/staff/:id

GET    /api/v1/drivers
POST   /api/v1/drivers
GET    /api/v1/vehicles
POST   /api/v1/vehicles

GET    /api/v1/trips
POST   /api/v1/trips
GET    /api/v1/trips/:id
PATCH  /api/v1/trips/:id
POST   /api/v1/trips/:id/cancel
POST   /api/v1/trips/:id/start
POST   /api/v1/trips/:id/complete
```

Action endpoints are acceptable when the operation represents a domain transition rather than a generic CRUD update.

`DELETE` on staff, drivers, and vehicles deactivates the record (status/active
flags) instead of removing the row, so historical trip references remain valid.

## Authentication

```text
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
POST   /api/v1/auth/password-reset/request
POST   /api/v1/auth/password-reset/confirm
```

Refresh tokens are opaque, stored server-side (Redis) with revocation support, and rotated on use. Access tokens are short-lived JWTs.

## Current User

```text
GET    /api/v1/me
GET    /api/v1/me/trips
```

`GET /api/v1/me/trips` returns only the caller's own transportation. Managers may additionally filter the full trip collection.

## Driver Execution

Driver-facing transitions on an assigned trip. All are idempotent and must authorize that the caller is the assigned driver.

```text
POST   /api/v1/trips/:id/stops/:stopId/arrive
POST   /api/v1/trips/:id/stops/:stopId/depart
POST   /api/v1/trips/:id/passengers/:passengerId/pickup
POST   /api/v1/trips/:id/passengers/:passengerId/no-show
```

## Location Ingestion

```text
POST   /api/v1/trips/:id/locations
```

Accepts a small batch of driver location samples while a trip is active. Each sample carries the device `recorded_at`; the server records `received_at`. Requests should be idempotent (see Idempotency below).

## Pagination

Collection endpoints should support pagination.

Example:

```text
GET /api/v1/trips?page=1&page_size=25
```

Response:

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "page_size": 25,
    "total": 120
  }
}
```

Cursor pagination can be introduced where high-volume telemetry requires it.

## Filtering

Use explicit filter parameters.

Example:

```text
GET /api/v1/trips?date=2026-09-21&status=scheduled&driver_id=123
```

Do not accept arbitrary SQL-like filter expressions from clients.

## Error Format

Use one consistent structure.

Example:

```json
{
  "error": {
    "code": "TRIP_CONFLICT",
    "message": "The selected driver is already assigned to another trip at this time.",
    "details": {}
  }
}
```

Do not expose stack traces or SQL errors to clients.

## Validation

Validate:

- required fields
- formats
- enum values
- date/time validity
- resource existence
- permissions
- domain constraints

Return `400` for malformed/invalid input and use appropriate conflict/status codes for business-state conflicts.

## Authorization

Examples:

- manager can manage fleet resources
- driver can read/update their own assigned operational trips
- staff can read their own transportation

A driver must not be able to access another driver's trips by changing an ID in the URL.

A staff member must not be able to query another staff member's private transportation information unless the product explicitly permits it.

## Idempotency

Use idempotency keys for mobile actions that may be retried, especially:

- pickup/no-show
- stop completion
- trip completion
- location batches where duplicate ingestion would matter

Example:

```http
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

## Date/Time

API timestamps should be ISO 8601 and preferably UTC.

User-facing timezone conversion happens at the UI boundary.
