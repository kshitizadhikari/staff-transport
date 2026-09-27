"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import {
  VEHICLE_STATUSES,
  type Vehicle,
  type VehicleInput,
} from "@/lib/directory";

const schema = z.object({
  registration_number: z
    .string()
    .min(1, "Registration number is required.")
    .max(50, "Registration number is too long."),
  model: z.string().max(200, "Model is too long."),
  capacity: z
    .number({ error: "Capacity must be a number." })
    .int("Capacity must be a whole number.")
    .min(0, "Capacity must be 0 or more."),
  status: z.enum(VEHICLE_STATUSES),
});

type Values = z.infer<typeof schema>;

type Props = {
  mode: "create" | "edit";
  vehicle?: Vehicle;
  onSubmit: (input: VehicleInput) => Promise<void>;
};

export function VehicleForm({ mode, vehicle, onSubmit }: Props) {
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      registration_number: vehicle?.registration_number ?? "",
      model: vehicle?.model ?? "",
      capacity: vehicle?.capacity ?? 0,
      status: vehicle?.status ?? "available",
    },
  });

  async function submit(values: Values) {
    try {
      await onSubmit({
        registration_number: values.registration_number,
        model: values.model || undefined,
        capacity: values.capacity,
        status: values.status,
      });
    } catch (error) {
      setError("root", {
        message:
          error instanceof ApiError
            ? error.message
            : "Unable to save. Please try again.",
      });
    }
  }

  return (
    <form onSubmit={handleSubmit(submit)} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor="registration_number">Registration number</Label>
        <Input
          id="registration_number"
          aria-invalid={!!errors.registration_number}
          {...register("registration_number")}
        />
        {errors.registration_number ? (
          <p className="text-sm text-destructive">
            {errors.registration_number.message}
          </p>
        ) : null}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="model">Model</Label>
          <Input id="model" {...register("model")} />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="capacity">Capacity</Label>
          <Input
            id="capacity"
            type="number"
            min={0}
            aria-invalid={!!errors.capacity}
            {...register("capacity", { valueAsNumber: true })}
          />
          {errors.capacity ? (
            <p className="text-sm text-destructive">{errors.capacity.message}</p>
          ) : null}
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="status">Status</Label>
        <select
          id="status"
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm"
          {...register("status")}
        >
          {VEHICLE_STATUSES.map((status) => (
            <option key={status} value={status}>
              {status}
            </option>
          ))}
        </select>
      </div>

      {errors.root ? (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errors.root.message}
        </p>
      ) : null}

      <div className="flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving…" : mode === "create" ? "Create vehicle" : "Save changes"}
        </Button>
      </div>
    </form>
  );
}
