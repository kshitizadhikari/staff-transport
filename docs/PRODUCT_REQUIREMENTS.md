# Product Requirements

## 1. Problem

An organization has one manager, many drivers, and many staff members. Staff need transportation between homes, offices, event venues, and other destinations.

Transportation is recurring and operationally coordinated. The manager needs visibility into assignments and execution.

## 2. User Stories

### Manager

- I can maintain the staff directory.
- I can maintain drivers and vehicles.
- I can create a trip for a date and time.
- I can assign a driver and vehicle.
- I can add multiple staff members to a trip.
- I can specify multiple ordered stops.
- I can create recurring transportation schedules.
- I can create event transportation plans.
- I can see which trips are scheduled, active, completed, cancelled, or delayed.
- I can see passenger pickup outcomes.
- I can inspect trip history.

### Driver

- I can see only trips relevant to me.
- I can see my current trip and next stop.
- I can open navigation for a stop.
- I can mark a staff member picked up or no-show.
- I can start and complete a trip.
- I can receive assignment/change notifications.
- My location can be shared during active operational trips when permitted.

### Staff

- I can see my upcoming transportation.
- I can see pickup time/location.
- I can see assigned driver/vehicle.
- I can see trip status.
- I can receive important transport notifications.

## 3. Functional Requirements

### Authentication

- Login/logout
- Refreshable sessions/tokens
- Role-based access control
- Password reset flow when password authentication is used

### Staff Management

Fields should include at minimum:

- name
- phone
- email, where available
- department
- active status
- home address
- geocoded home location
- optional default pickup preferences

### Driver Management

- name
- phone
- email, where available
- license number
- license expiry
- active status
- operational status

### Vehicle Management

- registration number
- model
- capacity
- active status
- optional compliance/maintenance fields

### Trip Management

- trip type
- date/time
- driver
- vehicle
- passengers
- ordered stops
- status
- notes
- optional event

### Trip Types

At minimum:

- `staff_pickup`
- `staff_dropoff`
- `office_to_event`
- `event_to_home`
- `custom`

### Recurring Schedules

Support weekly patterns and generate concrete trips.

A generated trip must be editable independently from the recurring schedule.

A schedule change must not silently rewrite historical trips that have already occurred.

### Event Transportation

An event may contain:

- name
- venue
- address/location
- start/end time
- participating staff
- transportation plans

Event transportation may generate multiple trips.

### Driver Execution

The driver flow must support ordered stop execution.

Example:

```text
Start Trip
  -> Navigate to Stop 1
  -> Picked up / No Show
  -> Navigate to Stop 2
  -> Picked up / No Show
  -> ...
  -> Complete Trip
```

### Notifications

Notifications should be sent for important changes, such as:

- trip assignment
- trip cancellation
- significant trip-time change
- driver started trip
- driver approaching/arrived where supported
- relevant passenger outcome

## 4. Status Model

### Trip

```text
scheduled
assigned
in_progress
completed
cancelled
```

The system may introduce `delayed` as a derived operational state rather than a persisted primary state if that is sufficient.

### Trip Passenger

```text
assigned
picked_up
no_show
completed
cancelled
```

### Driver

```text
available
on_trip
offline
leave
inactive
```

### Vehicle

```text
available
assigned
maintenance
inactive
```

## 5. Business Rules

1. A driver should not be simultaneously assigned to conflicting active trips unless the product explicitly supports multi-leg scheduling.
2. A vehicle should not be assigned to conflicting trips.
3. A trip's passenger count should not exceed vehicle capacity when a vehicle with a known capacity is assigned.
4. A cancelled trip cannot later be silently completed.
5. A completed trip should preserve its historical driver, vehicle, passengers, and stop information.
6. A passenger can be a participant in many trips over time.
7. A trip can have many passengers and many ordered stops.
8. A staff member's current home address must not overwrite historical trip-stop data where historical accuracy matters.
9. Recurring schedules create concrete trips; concrete trip changes must be possible without corrupting the recurrence rule.
10. Driver location is operational telemetry, not a permanent requirement for all time.

## 6. MVP Acceptance Criteria

### Manager

Can create staff, drivers, vehicles, and a trip, assign resources, and monitor its status.

### Driver

Can see assigned trips, navigate to stops, record passenger outcomes, and complete a trip.

### Staff

Can view today's/upcoming trips and their current status.

### Reliability

A failed notification or location update must not corrupt core trip state.

## 7. Future Requirements

Potential later additions:

- route optimization
- live map
- ETA calculation
- offline-first driver mode
- trip templates
- fuel tracking
- vehicle maintenance
- driver performance reports
- attendance integration
- payroll integration
- multi-office organizations
- granular permissions
