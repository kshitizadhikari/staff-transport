import { apiFetch } from "@/lib/api";

export type Paginated<T> = {
  data: T[];
  pagination: { page: number; page_size: number; total: number };
};

export type ListParams = {
  search?: string;
  status?: string;
  active?: boolean;
  page?: number;
  page_size?: number;
};

function toQuery(
  params: Record<string, string | number | boolean | undefined>,
): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

// --- Staff ---

export type Staff = {
  id: string;
  user_id: string;
  name: string;
  email?: string;
  phone?: string;
  department?: string;
  home_address?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type StaffInput = {
  name: string;
  email?: string;
  phone?: string;
  department?: string;
  home_address?: string;
  password?: string;
  active?: boolean;
};

export const staffApi = {
  list: (params: ListParams = {}) =>
    apiFetch<Paginated<Staff>>(`/staff${toQuery(params)}`),
  get: (id: string) => apiFetch<Staff>(`/staff/${id}`),
  create: (input: StaffInput) =>
    apiFetch<Staff>("/staff", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: StaffInput) =>
    apiFetch<Staff>(`/staff/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  deactivate: (id: string) =>
    apiFetch<void>(`/staff/${id}`, { method: "DELETE" }),
};

// --- Drivers ---

export const DRIVER_STATUSES = [
  "available",
  "on_trip",
  "offline",
  "leave",
  "inactive",
] as const;

export type DriverStatus = (typeof DRIVER_STATUSES)[number];

export type Driver = {
  id: string;
  user_id: string;
  name: string;
  email?: string;
  phone?: string;
  license_number?: string;
  license_expiry?: string;
  status: DriverStatus;
  created_at: string;
  updated_at: string;
};

export type DriverInput = {
  name: string;
  email?: string;
  phone?: string;
  license_number?: string;
  license_expiry?: string;
  status?: DriverStatus;
  password?: string;
};

export const driverApi = {
  list: (params: ListParams = {}) =>
    apiFetch<Paginated<Driver>>(`/drivers${toQuery(params)}`),
  get: (id: string) => apiFetch<Driver>(`/drivers/${id}`),
  create: (input: DriverInput) =>
    apiFetch<Driver>("/drivers", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: DriverInput) =>
    apiFetch<Driver>(`/drivers/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  deactivate: (id: string) =>
    apiFetch<void>(`/drivers/${id}`, { method: "DELETE" }),
};

// --- Vehicles ---

export const VEHICLE_STATUSES = [
  "available",
  "assigned",
  "maintenance",
  "inactive",
] as const;

export type VehicleStatus = (typeof VEHICLE_STATUSES)[number];

export type Vehicle = {
  id: string;
  registration_number: string;
  model?: string;
  capacity: number;
  status: VehicleStatus;
  created_at: string;
  updated_at: string;
};

export type VehicleInput = {
  registration_number: string;
  model?: string;
  capacity: number;
  status?: VehicleStatus;
};

export const vehicleApi = {
  list: (params: ListParams = {}) =>
    apiFetch<Paginated<Vehicle>>(`/vehicles${toQuery(params)}`),
  get: (id: string) => apiFetch<Vehicle>(`/vehicles/${id}`),
  create: (input: VehicleInput) =>
    apiFetch<Vehicle>("/vehicles", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (id: string, input: VehicleInput) =>
    apiFetch<Vehicle>(`/vehicles/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  deactivate: (id: string) =>
    apiFetch<void>(`/vehicles/${id}`, { method: "DELETE" }),
};
