-- 0001_init.sql
-- Initial schema for the staff transport management system.
-- PostgreSQL + PostGIS. All timestamps are stored as timestamptz (UTC).

CREATE EXTENSION IF NOT EXISTS postgis;

-- ---------------------------------------------------------------------------
-- Helpers
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text NOT NULL,
    email            text,
    phone            text,
    password_hash    text,
    external_auth_id text,
    role             text NOT NULL CHECK (role IN ('manager', 'driver', 'staff')),
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX users_role_idx ON users (role);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- staff
-- ---------------------------------------------------------------------------

CREATE TABLE staff (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE RESTRICT,
    department   text,
    home_address text,
    home_point   geometry(Point, 4326),
    active       boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX staff_home_point_idx ON staff USING gist (home_point);

CREATE TRIGGER staff_set_updated_at
    BEFORE UPDATE ON staff
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- drivers
-- ---------------------------------------------------------------------------

CREATE TABLE drivers (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE RESTRICT,
    license_number text,
    license_expiry date,
    status         text NOT NULL DEFAULT 'offline'
        CHECK (status IN ('available', 'on_trip', 'offline', 'leave', 'inactive')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER drivers_set_updated_at
    BEFORE UPDATE ON drivers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- vehicles
-- ---------------------------------------------------------------------------

CREATE TABLE vehicles (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_number text NOT NULL UNIQUE,
    model               text,
    capacity            integer NOT NULL DEFAULT 0 CHECK (capacity >= 0),
    status              text NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'assigned', 'maintenance', 'inactive')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER vehicles_set_updated_at
    BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- events
-- ---------------------------------------------------------------------------

CREATE TABLE events (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    address    text,
    point      geometry(Point, 4326),
    starts_at  timestamptz,
    ends_at    timestamptz,
    notes      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX events_point_idx ON events USING gist (point);
CREATE INDEX events_starts_at_idx ON events (starts_at);

CREATE TRIGGER events_set_updated_at
    BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- trips
-- ---------------------------------------------------------------------------

CREATE TABLE trips (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type               text NOT NULL
        CHECK (type IN ('staff_pickup', 'staff_dropoff', 'office_to_event', 'event_to_home', 'custom')),
    scheduled_start_at timestamptz NOT NULL,
    driver_id          uuid REFERENCES drivers (id) ON DELETE SET NULL,
    vehicle_id         uuid REFERENCES vehicles (id) ON DELETE SET NULL,
    event_id           uuid REFERENCES events (id) ON DELETE SET NULL,
    status             text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'assigned', 'in_progress', 'completed', 'cancelled')),
    notes              text,
    started_at         timestamptz,
    completed_at       timestamptz,
    cancelled_at       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX trips_scheduled_start_at_idx ON trips (scheduled_start_at);
CREATE INDEX trips_driver_id_scheduled_start_at_idx ON trips (driver_id, scheduled_start_at);
CREATE INDEX trips_vehicle_id_scheduled_start_at_idx ON trips (vehicle_id, scheduled_start_at);
CREATE INDEX trips_status_scheduled_start_at_idx ON trips (status, scheduled_start_at);
CREATE INDEX trips_event_id_idx ON trips (event_id);

CREATE TRIGGER trips_set_updated_at
    BEFORE UPDATE ON trips
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- trip_stops
-- ---------------------------------------------------------------------------

CREATE TABLE trip_stops (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id      uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    "sequence"   integer NOT NULL CHECK ("sequence" >= 0),
    stop_type    text NOT NULL CHECK (stop_type IN ('pickup', 'dropoff', 'event', 'office', 'custom')),
    address      text,
    point        geometry(Point, 4326),
    scheduled_at timestamptz,
    arrived_at   timestamptz,
    departed_at  timestamptz,
    status       text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'arrived', 'departed', 'skipped')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_id, "sequence")
);

CREATE INDEX trip_stops_trip_id_sequence_idx ON trip_stops (trip_id, "sequence");
CREATE INDEX trip_stops_point_idx ON trip_stops USING gist (point);

CREATE TRIGGER trip_stops_set_updated_at
    BEFORE UPDATE ON trip_stops
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- trip_passengers
-- ---------------------------------------------------------------------------

CREATE TABLE trip_passengers (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id          uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    staff_id         uuid NOT NULL REFERENCES staff (id) ON DELETE RESTRICT,
    pickup_stop_id   uuid REFERENCES trip_stops (id) ON DELETE SET NULL,
    dropoff_stop_id  uuid REFERENCES trip_stops (id) ON DELETE SET NULL,
    status           text NOT NULL DEFAULT 'assigned'
        CHECK (status IN ('assigned', 'picked_up', 'no_show', 'completed', 'cancelled')),
    picked_up_at     timestamptz,
    completed_at     timestamptz,
    notes            text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_id, staff_id)
);

CREATE INDEX trip_passengers_staff_id_idx ON trip_passengers (staff_id);
CREATE INDEX trip_passengers_trip_id_idx ON trip_passengers (trip_id);

CREATE TRIGGER trip_passengers_set_updated_at
    BEFORE UPDATE ON trip_passengers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- event_staff / event_trips
-- ---------------------------------------------------------------------------

CREATE TABLE event_staff (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    staff_id   uuid NOT NULL REFERENCES staff (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, staff_id)
);

CREATE INDEX event_staff_staff_id_idx ON event_staff (staff_id);

CREATE TABLE event_trips (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, trip_id)
);

CREATE INDEX event_trips_trip_id_idx ON event_trips (trip_id);

-- ---------------------------------------------------------------------------
-- recurring_schedules / recurring_schedule_trips
-- ---------------------------------------------------------------------------

CREATE TABLE recurring_schedules (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text NOT NULL,
    trip_type        text NOT NULL
        CHECK (trip_type IN ('staff_pickup', 'staff_dropoff', 'office_to_event', 'event_to_home', 'custom')),
    timezone         text NOT NULL DEFAULT 'UTC',
    active           boolean NOT NULL DEFAULT true,
    start_date       date NOT NULL,
    end_date         date,
    recurrence_rule  text NOT NULL,
    template         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER recurring_schedules_set_updated_at
    BEFORE UPDATE ON recurring_schedules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE recurring_schedule_trips (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recurring_schedule_id uuid NOT NULL REFERENCES recurring_schedules (id) ON DELETE CASCADE,
    trip_id              uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    occurrence_date      date NOT NULL,
    generated_at         timestamptz NOT NULL DEFAULT now(),
    created_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (recurring_schedule_id, occurrence_date)
);

-- ---------------------------------------------------------------------------
-- refresh_tokens
-- ---------------------------------------------------------------------------

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

-- ---------------------------------------------------------------------------
-- driver_locations
-- ---------------------------------------------------------------------------

CREATE TABLE driver_locations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id   uuid NOT NULL REFERENCES drivers (id) ON DELETE CASCADE,
    trip_id     uuid REFERENCES trips (id) ON DELETE SET NULL,
    point       geometry(Point, 4326) NOT NULL,
    recorded_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    accuracy_m  double precision,
    UNIQUE (driver_id, recorded_at)
);

CREATE INDEX driver_locations_driver_id_recorded_at_idx ON driver_locations (driver_id, recorded_at);
CREATE INDEX driver_locations_trip_id_recorded_at_idx ON driver_locations (trip_id, recorded_at);
CREATE INDEX driver_locations_point_idx ON driver_locations USING gist (point);

-- ---------------------------------------------------------------------------
-- notifications
-- ---------------------------------------------------------------------------

CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}'::jsonb,
    status     text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    sent_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_id_status_idx ON notifications (user_id, status);

-- ---------------------------------------------------------------------------
-- audit_logs
-- ---------------------------------------------------------------------------

CREATE TABLE audit_logs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    action        text NOT NULL,
    entity_type   text NOT NULL,
    entity_id     uuid,
    metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_entity_type_entity_id_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_actor_user_id_idx ON audit_logs (actor_user_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);
