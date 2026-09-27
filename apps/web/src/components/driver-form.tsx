"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import {
  DRIVER_STATUSES,
  type Driver,
  type DriverInput,
} from "@/lib/directory";

const schema = z.object({
  name: z.string().min(1, "Name is required."),
  email: z.union([z.literal(""), z.email("Enter a valid email address.")]),
  phone: z.string().max(50, "Phone is too long."),
  license_number: z.string().max(100, "License number is too long."),
  license_expiry: z.union([
    z.literal(""),
    z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "Use YYYY-MM-DD."),
  ]),
  status: z.enum(DRIVER_STATUSES),
  password: z.union([
    z.literal(""),
    z.string().min(8, "Password must be at least 8 characters."),
  ]),
});

type Values = z.infer<typeof schema>;

type Props = {
  mode: "create" | "edit";
  driver?: Driver;
  onSubmit: (input: DriverInput) => Promise<void>;
};

export function DriverForm({ mode, driver, onSubmit }: Props) {
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: driver?.name ?? "",
      email: driver?.email ?? "",
      phone: driver?.phone ?? "",
      license_number: driver?.license_number ?? "",
      license_expiry: driver?.license_expiry ?? "",
      status: driver?.status ?? "offline",
      password: "",
    },
  });

  async function submit(values: Values) {
    const input: DriverInput = {
      name: values.name.trim(),
      email: values.email || undefined,
      phone: values.phone || undefined,
      license_number: values.license_number || undefined,
      license_expiry: values.license_expiry || undefined,
      status: values.status,
    };
    if (mode === "create") {
      input.password = values.password || undefined;
    }

    try {
      await onSubmit(input);
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
        <Label htmlFor="name">Name</Label>
        <Input id="name" aria-invalid={!!errors.name} {...register("name")} />
        {errors.name ? (
          <p className="text-sm text-destructive">{errors.name.message}</p>
        ) : null}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="email">Email</Label>
          <Input id="email" type="email" aria-invalid={!!errors.email} {...register("email")} />
          {errors.email ? (
            <p className="text-sm text-destructive">{errors.email.message}</p>
          ) : null}
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="phone">Phone</Label>
          <Input id="phone" {...register("phone")} />
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="license_number">License number</Label>
          <Input id="license_number" {...register("license_number")} />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="license_expiry">License expiry</Label>
          <Input id="license_expiry" type="date" aria-invalid={!!errors.license_expiry} {...register("license_expiry")} />
          {errors.license_expiry ? (
            <p className="text-sm text-destructive">
              {errors.license_expiry.message}
            </p>
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
          {DRIVER_STATUSES.map((status) => (
            <option key={status} value={status}>
              {status.replace("_", " ")}
            </option>
          ))}
        </select>
      </div>

      {mode === "create" ? (
        <div className="flex flex-col gap-2">
          <Label htmlFor="password">Initial password (optional)</Label>
          <Input
            id="password"
            type="password"
            autoComplete="new-password"
            aria-invalid={!!errors.password}
            {...register("password")}
          />
          {errors.password ? (
            <p className="text-sm text-destructive">{errors.password.message}</p>
          ) : null}
        </div>
      ) : null}

      {errors.root ? (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errors.root.message}
        </p>
      ) : null}

      <div className="flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving…" : mode === "create" ? "Create driver" : "Save changes"}
        </Button>
      </div>
    </form>
  );
}
