export type TripStatus =
  | 'scheduled'
  | 'assigned'
  | 'in_progress'
  | 'completed'
  | 'cancelled';

export type TripStop = {
  id: string;
  sequence: number;
  stopType: 'pickup' | 'dropoff' | 'event' | 'office' | 'custom';
  address: string;
  scheduledAt: string | null;
  status: 'pending' | 'arrived' | 'departed' | 'skipped';
};

export type TripSummary = {
  id: string;
  type: string;
  status: TripStatus;
  scheduledStartAt: string;
  vehicle: string | null;
  stopCount: number;
  passengerCount: number;
};

export type TripDetail = TripSummary & {
  stops: TripStop[];
  passengers: {
    id: string;
    name: string;
    status: 'assigned' | 'picked_up' | 'no_show' | 'completed' | 'cancelled';
  }[];
};
