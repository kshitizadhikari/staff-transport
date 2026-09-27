import { apiFetch } from "@/lib/api";
import type { Paginated } from "@/lib/directory";

export const TRIP_TYPES = [
  "staff_pickup",
  "staff_dropoff",
  "office_to_event",
  "event_to_home",
  "custom",
] as const;

export const STOP_TYPES = [
  "pickup",
  "dropoff",
  "event",
  "office",
  "custom",
] as const;

export const TRIP_STATUSES = [
  "scheduled",
  "assigned",
  "in_progress",
  "completed",
  "cancelled",
] as const;

export type TripType = (typeof TRIP_TYPES)[number];
export type StopType = (typeof STOP_TYPES)[number];
export type TripStatus = (typeof TRIP_STATUSES)[number];

export type TripStop = {
  id: string;
  sequence: number;
  type: StopType;
  address?: string;
  scheduled_at?: string;
  status: "pending" | "arrived" | "departed" | "skipped";
};

export type TripPassenger = {
  id: string;
  staff_id: string;
  name: string;
  pickup_stop_id?: string;
  dropoff_stop_id?: string;
  status: "assigned" | "picked_up" | "no_show" | "completed" | "cancelled";
};

export type Trip = {
  id: string;
  type: TripType;
  scheduled_start_at: string;
  driver_id?: string;
  driver_name?: string;
  vehicle_id?: string;
  vehicle_registration?: string;
  event_id?: string;
  status: TripStatus;
  notes?: string;
  started_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  stops?: TripStop[];
  passengers?: TripPassenger[];
  created_at: string;
  updated_at: string;
};

export type StopInput = {
  type: StopType;
  address?: string;
  scheduled_at?: string;
};

export type PassengerInput = {
  staff_id: string;
  pickup_stop_index?: number;
  dropoff_stop_index?: number;
};

export type TripInput = {
  type: TripType;
  scheduled_start_at: string;
  driver_id?: string;
  vehicle_id?: string;
  event_id?: string;
  notes?: string;
  stops: StopInput[];
  passengers: PassengerInput[];
};

export type TripUpdateInput = {
  type?: TripType;
  scheduled_start_at?: string;
  driver_id?: string;
  vehicle_id?: string;
  notes?: string;
  passengers?: PassengerInput[];
};

export type TripListParams = {
  status?: string;
  driver_id?: string;
  vehicle_id?: string;
  date?: string;
  page?: number;
  page_size?: number;
};

function query(params: TripListParams): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const value = search.toString();
  return value ? `?${value}` : "";
}

export const tripApi = {
  list: (params: TripListParams = {}) =>
    apiFetch<Paginated<Trip>>(`/trips${query(params)}`),
  get: (id: string) => apiFetch<Trip>(`/trips/${id}`),
  create: (input: TripInput) =>
    apiFetch<Trip>("/trips", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: TripUpdateInput) =>
    apiFetch<Trip>(`/trips/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  cancel: (id: string) =>
    apiFetch<void>(`/trips/${id}/cancel`, { method: "POST" }),
};

// datetime-local helpers: the API uses RFC3339 (UTC), the input uses local time.
export function toLocalInput(iso?: string): string {
  const date = iso ? new Date(iso) : new Date();
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function fromLocalInput(value: string): string {
  return new Date(value).toISOString();
}
