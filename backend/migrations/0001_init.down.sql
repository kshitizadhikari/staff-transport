-- 0001_init.down.sql
-- Reverts the initial schema. Drops objects in reverse dependency order.

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS driver_locations;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS recurring_schedule_trips;
DROP TABLE IF EXISTS recurring_schedules;
DROP TABLE IF EXISTS event_trips;
DROP TABLE IF EXISTS event_staff;
DROP TABLE IF EXISTS trip_passengers;
DROP TABLE IF EXISTS trip_stops;
DROP TABLE IF EXISTS trips;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS vehicles;
DROP TABLE IF EXISTS drivers;
DROP TABLE IF EXISTS staff;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();

-- The postgis extension is intentionally left installed; other databases or
-- future migrations may depend on it.
