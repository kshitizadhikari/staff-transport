"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { driverApi, staffApi, vehicleApi } from "@/lib/directory";
import {
  STOP_TYPES,
  TRIP_TYPES,
  toLocalInput,
  fromLocalInput,
  type TripInput,
} from "@/lib/trips";

const stopSchema = z.object({
  type: z.enum(STOP_TYPES),
  address: z.string().max(500, "Address is too long."),
});

const passengerSchema = z.object({
  staff_id: z.string().min(1, "Select a staff member."),
  pickup_stop_index: z.string(),
  dropoff_stop_index: z.string(),
});

const schema = z.object({
  type: z.enum(TRIP_TYPES),
  scheduled_start_at: z.string().min(1, "Schedule is required."),
  driver_id: z.string(),
  vehicle_id: z.string(),
  notes: z.string().max(2000, "Notes are too long."),
  stops: z.array(stopSchema).min(1, "Add at least one stop."),
  passengers: z.array(passengerSchema),
});

type Values = z.infer<typeof schema>;

export function TripForm({
  onSubmit,
}: {
  onSubmit: (input: TripInput) => Promise<void>;
}) {
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      type: "staff_pickup",
      scheduled_start_at: toLocalInput(),
      driver_id: "",
      vehicle_id: "",
      notes: "",
      stops: [
        { type: "pickup", address: "" },
        { type: "dropoff", address: "" },
      ],
      passengers: [],
    },
  });

  const stopsArray = useFieldArray({ control, name: "stops" });
  const passengersArray = useFieldArray({ control, name: "passengers" });
  const watchedStops = useWatch({ control, name: "stops" });
  const stopCount = watchedStops?.length ?? 0;

  const staffQuery = useQuery({
    queryKey: ["staff", "options"],
    queryFn: () => staffApi.list({ active: true, page_size: 100 }),
  });
  const driversQuery = useQuery({
    queryKey: ["drivers", "options"],
    queryFn: () => driverApi.list({ page_size: 100 }),
  });
  const vehiclesQuery = useQuery({
    queryKey: ["vehicles", "options"],
    queryFn: () => vehicleApi.list({ page_size: 100 }),
  });

  async function submit(values: Values) {
    const input: TripInput = {
      type: values.type,
      scheduled_start_at: fromLocalInput(values.scheduled_start_at),
      driver_id: values.driver_id || undefined,
      vehicle_id: values.vehicle_id || undefined,
      notes: values.notes || undefined,
      stops: values.stops.map((stop) => ({
        type: stop.type,
        address: stop.address || undefined,
      })),
      passengers: values.passengers.map((passenger) => ({
        staff_id: passenger.staff_id,
        pickup_stop_index: indexOrUndefined(passenger.pickup_stop_index),
        dropoff_stop_index: indexOrUndefined(passenger.dropoff_stop_index),
      })),
    };

    try {
      await onSubmit(input);
    } catch (error) {
      setError("root", {
        message:
          error instanceof ApiError
            ? error.message
            : "Unable to save trip. Please try again.",
      });
    }
  }

  return (
    <form onSubmit={handleSubmit(submit)} className="flex flex-col gap-6" noValidate>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="type">Trip type</Label>
          <select
            id="type"
            className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
            {...register("type")}>
            {TRIP_TYPES.map((type) => (
              <option key={type} value={type}>
                {type.replace(/_/g, " ")}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="scheduled_start_at">Scheduled start</Label>
          <Input
            id="scheduled_start_at"
            type="datetime-local"
            aria-invalid={!!errors.scheduled_start_at}
            {...register("scheduled_start_at")}
          />
          {errors.scheduled_start_at ? (
            <p className="text-sm text-destructive">
              {errors.scheduled_start_at.message}
            </p>
          ) : null}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="driver_id">Driver</Label>
          <select
            id="driver_id"
            className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
            {...register("driver_id")}>
            <option value="">Unassigned</option>
            {(driversQuery.data?.data ?? []).map((driver) => (
              <option key={driver.id} value={driver.id}>
                {driver.name}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="vehicle_id">Vehicle</Label>
          <select
            id="vehicle_id"
            className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
            {...register("vehicle_id")}>
            <option value="">Unassigned</option>
            {(vehiclesQuery.data?.data ?? []).map((vehicle) => (
              <option key={vehicle.id} value={vehicle.id}>
                {vehicle.registration_number} ({vehicle.capacity} seats)
              </option>
            ))}
          </select>
        </div>
      </div>

      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">Stops</h2>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => stopsArray.append({ type: "pickup", address: "" })}>
            Add stop
          </Button>
        </div>
        {errors.stops?.message ? (
          <p className="text-sm text-destructive">{errors.stops.message}</p>
        ) : null}
        {stopsArray.fields.map((field, index) => (
          <div
            key={field.id}
            className="flex flex-wrap items-end gap-3 rounded-lg bg-muted/40 p-3">
            <span className="text-sm text-muted-foreground">#{index + 1}</span>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`stops.${index}.type`}>Type</Label>
              <select
                id={`stops.${index}.type`}
                className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
                {...register(`stops.${index}.type`)}>
                {STOP_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex min-w-48 flex-1 flex-col gap-2">
              <Label htmlFor={`stops.${index}.address`}>Address</Label>
              <Input
                id={`stops.${index}.address`}
                {...register(`stops.${index}.address`)}
              />
            </div>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              disabled={stopsArray.fields.length <= 1}
              onClick={() => stopsArray.remove(index)}>
              Remove
            </Button>
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">Passengers</h2>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              passengersArray.append({
                staff_id: "",
                pickup_stop_index: "",
                dropoff_stop_index: "",
              })
            }>
            Add passenger
          </Button>
        </div>
        {passengersArray.fields.map((field, index) => (
          <div
            key={field.id}
            className="flex flex-wrap items-end gap-3 rounded-lg bg-muted/40 p-3">
            <div className="flex min-w-48 flex-1 flex-col gap-2">
              <Label htmlFor={`passengers.${index}.staff_id`}>Staff</Label>
              <select
                id={`passengers.${index}.staff_id`}
                className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
                {...register(`passengers.${index}.staff_id`)}>
                <option value="">Select staff…</option>
                {(staffQuery.data?.data ?? []).map((member) => (
                  <option key={member.id} value={member.id}>
                    {member.name}
                  </option>
                ))}
              </select>
              {errors.passengers?.[index]?.staff_id ? (
                <p className="text-sm text-destructive">
                  {errors.passengers[index]?.staff_id?.message}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`passengers.${index}.pickup_stop_index`}>
                Pickup stop
              </Label>
              <select
                id={`passengers.${index}.pickup_stop_index`}
                className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
                {...register(`passengers.${index}.pickup_stop_index`)}>
                <option value="">None</option>
                {stopOptions(stopCount)}
              </select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`passengers.${index}.dropoff_stop_index`}>
                Dropoff stop
              </Label>
              <select
                id={`passengers.${index}.dropoff_stop_index`}
                className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
                {...register(`passengers.${index}.dropoff_stop_index`)}>
                <option value="">None</option>
                {stopOptions(stopCount)}
              </select>
            </div>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => passengersArray.remove(index)}>
              Remove
            </Button>
          </div>
        ))}
      </section>

      <div className="flex flex-col gap-2">
        <Label htmlFor="notes">Notes</Label>
        <textarea
          id="notes"
          rows={3}
          className="rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm"
          {...register("notes")}
        />
      </div>

      {errors.root ? (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errors.root.message}
        </p>
      ) : null}

      <div className="flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Creating…" : "Create trip"}
        </Button>
      </div>
    </form>
  );
}

function stopOptions(count: number) {
  return Array.from({ length: count }, (_, index) => (
    <option key={index} value={String(index)}>
      Stop {index + 1}
    </option>
  ));
}

function indexOrUndefined(value: string): number | undefined {
  return value === "" ? undefined : Number(value);
}
