"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import type { Staff, StaffInput } from "@/lib/directory";

const schema = z.object({
  name: z.string().min(1, "Name is required."),
  email: z.union([z.literal(""), z.email("Enter a valid email address.")]),
  phone: z.string().max(50, "Phone is too long."),
  department: z.string().max(200, "Department is too long."),
  home_address: z.string().max(500, "Address is too long."),
  password: z.union([
    z.literal(""),
    z.string().min(8, "Password must be at least 8 characters."),
  ]),
  active: z.boolean(),
});

type Values = z.infer<typeof schema>;

type Props = {
  mode: "create" | "edit";
  staff?: Staff;
  onSubmit: (input: StaffInput) => Promise<void>;
};

export function StaffForm({ mode, staff, onSubmit }: Props) {
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: staff?.name ?? "",
      email: staff?.email ?? "",
      phone: staff?.phone ?? "",
      department: staff?.department ?? "",
      home_address: staff?.home_address ?? "",
      password: "",
      active: staff?.active ?? true,
    },
  });

  async function submit(values: Values) {
    const input: StaffInput = {
      name: values.name.trim(),
      email: values.email || undefined,
      phone: values.phone || undefined,
      department: values.department || undefined,
      home_address: values.home_address || undefined,
      active: values.active,
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
          <Input id="phone" aria-invalid={!!errors.phone} {...register("phone")} />
          {errors.phone ? (
            <p className="text-sm text-destructive">{errors.phone.message}</p>
          ) : null}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2">
          <Label htmlFor="department">Department</Label>
          <Input id="department" {...register("department")} />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="home_address">Home address</Label>
          <Input id="home_address" {...register("home_address")} />
        </div>
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
          ) : (
            <p className="text-xs text-muted-foreground">
              Leave blank to create the account without login credentials.
            </p>
          )}
        </div>
      ) : null}

      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="size-4 rounded border-input"
          {...register("active")}
        />
        Active
      </label>

      {errors.root ? (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errors.root.message}
        </p>
      ) : null}

      <div className="flex justify-end gap-2">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving…" : mode === "create" ? "Create staff" : "Save changes"}
        </Button>
      </div>
    </form>
  );
}
