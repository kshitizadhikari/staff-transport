# Database Design

PostgreSQL is the system of record. Use PostGIS for geographic locations.

## Core Tables

### users

Authentication identity and role.

```text
id
name
email
phone
password_hash / external_auth_id
role
status
created_at
updated_at
```

Roles:

```text
manager
driver
staff
```

### staff

Staff-specific profile information.

```text
id
user_id
department
home_address
home_point
active
created_at
updated_at
```

### drivers

Driver-specific profile information.

```text
id
user_id
license_number
license_expiry
status
created_at
updated_at
```

### vehicles

```text
id
registration_number
model
capacity
status
created_at
updated_at
```

### trips

```text
id
type
scheduled_start_at
driver_id
vehicle_id
event_id
status
notes
started_at
completed_at
cancelled_at
created_at
updated_at
```

### trip_stops

```text
id
trip_id
sequence
stop_type
address
point
scheduled_at
arrived_at
departed_at
status
created_at
updated_at
```

Potential stop types:

```text
pickup
dropoff
event
office
custom
```

### trip_passengers

```text
id
trip_id
staff_id
pickup_stop_id
dropoff_stop_id
status
picked_up_at
completed_at
notes
created_at
updated_at
```

This relationship is critical. A passenger assignment is not the same thing as the passenger's staff profile.

### events

```text
id
name
address
point
starts_at
ends_at
notes
created_at
updated_at
```

### event_staff

Participants of an event. Separate from trip passengers.

```text
id
event_id
staff_id
created_at
```

### event_trips

Links events to the trips that serve them. An event may generate multiple trips.

```text
id
event_id
trip_id
created_at
```

### recurring_schedules

```text
id
name
trip_type
timezone
active
start_date
end_date
recurrence_rule
template
created_at
updated_at
```

`template` stores the stop/passenger shape used to generate concrete trips. Avoid storing only a vague `days = "MWF"` field if a standards-based recurrence representation can be used safely.

### recurring_schedule_trips

Links a recurring schedule to the concrete trips it generated. Used to make generation idempotent.

```text
id
recurring_schedule_id
trip_id
occurrence_date
generated_at
created_at
```

A unique constraint on `(recurring_schedule_id, occurrence_date)` prevents duplicate trip generation on job retry.

### refresh_tokens

Optional durable audit of refresh-token issuance. Live refresh-token state and revocation are stored in Redis.

```text
id
user_id
token_hash
expires_at
revoked_at
created_at
```

### driver_locations

```text
id
driver_id
trip_id
point
recorded_at
received_at
accuracy_m
```

### notifications

```text
id
user_id
type
payload
status
sent_at
created_at
```

### audit_logs

```text
id
actor_user_id
action
entity_type
entity_id
metadata
created_at
```

## Important Indexes

At minimum, consider indexes on:

```text
trips(scheduled_start_at)
trips(driver_id, scheduled_start_at)
trips(vehicle_id, scheduled_start_at)
trips(status, scheduled_start_at)
trips(event_id)
trip_passengers(staff_id)
trip_passengers(trip_id)
trip_stops(trip_id, sequence)
driver_locations(driver_id, recorded_at)
driver_locations(trip_id, recorded_at)
notifications(user_id, status)
audit_logs(entity_type, entity_id)
event_staff(staff_id)
event_trips(trip_id)
recurring_schedule_trips(recurring_schedule_id, occurrence_date)  -- unique
```

Use PostGIS spatial indexes for location columns.

## Historical Data

A staff member can change address.

Historical trips should not unexpectedly change because the current staff profile changed.

Therefore trip stops should contain the location snapshot actually used for that trip.

## Soft Delete

Do not blindly apply soft delete to every table.

Prefer statuses such as `active/inactive` for users, drivers, and vehicles where preserving historical relationships is important.

Trips and audit records should generally remain immutable historical records after completion except for explicitly supported corrections.
