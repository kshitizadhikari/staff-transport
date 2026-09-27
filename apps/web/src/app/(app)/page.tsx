"use client";

import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/lib/auth";

type HealthResponse = { status: string };

const workspaces = [
  {
    title: "Manager",
    description:
      "Plan trips, assign drivers and vehicles, manage staff, and monitor execution.",
  },
  {
    title: "Driver",
    description:
      "Execute assigned trips from the mobile app: ordered stops, pickups, no-shows.",
  },
  {
    title: "Staff",
    description:
      "View upcoming transportation, assigned driver/vehicle, and trip status.",
  },
];

export default function Home() {
  const { user } = useAuth();

  const health = useQuery({
    queryKey: ["health"],
    queryFn: () => apiFetch<HealthResponse>("/health"),
    retry: false,
  });

  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-semibold tracking-tight">
            Welcome, {user?.name}
          </h1>
          <p className="text-muted-foreground">
            Manager-driven transport operations.
          </p>
        </div>
        <Badge variant={health.isSuccess ? "default" : "secondary"}>
          {health.isSuccess
            ? "API online"
            : health.isError
              ? "API offline"
              : "Checking API"}
        </Badge>
      </header>

      <div className="grid gap-4 sm:grid-cols-3">
        {workspaces.map((workspace) => (
          <Card key={workspace.title}>
            <CardHeader>
              <CardTitle>{workspace.title}</CardTitle>
              <CardDescription>{workspace.description}</CardDescription>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Next steps</CardTitle>
          <CardDescription>
            Manage your staff directory, drivers, and vehicles from the
            navigation above. Trip planning is coming next.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Seed a manager account with{" "}
          <code className="rounded bg-muted px-1.5 py-0.5">
            make seed-manager
          </code>
          .
        </CardContent>
      </Card>
    </div>
  );
}
