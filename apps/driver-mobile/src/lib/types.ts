export type TripStatus =
  | 'scheduled'
  | 'assigned'
  | 'in_progress'
  | 'completed'
  | 'cancelled';

export type StopStatus = 'pending' | 'arrived' | 'departed' | 'skipped';

export type PassengerStatus =
  | 'assigned'
  | 'picked_up'
  | 'no_show'
  | 'completed'
  | 'cancelled';

export type Role = 'manager' | 'driver' | 'staff';

export type AuthUser = {
  id: string;
  name: string;
  email?: string;
  phone?: string;
  role: Role;
};

export type TripStop = {
  id: string;
  sequence: number;
  type: 'pickup' | 'dropoff' | 'event' | 'office' | 'custom';
  address?: string;
  scheduled_at?: string;
  status: StopStatus;
};

export type TripPassenger = {
  id: string;
  staff_id: string;
  name: string;
  pickup_stop_id?: string;
  dropoff_stop_id?: string;
  status: PassengerStatus;
};

export type Trip = {
  id: string;
  type: string;
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

export type Paginated<T> = {
  data: T[];
  pagination: { page: number; page_size: number; total: number };
};
