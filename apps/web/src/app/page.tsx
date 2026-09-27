"use client";

import { useQuery } from "@tanstack/react-query";

import { RequireAuth } from "@/components/require-auth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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

function Dashboard() {
  const { user, logout } = useAuth();

  const health = useQuery({
    queryKey: ["health"],
    queryFn: () => apiFetch<HealthResponse>("/health"),
    retry: false,
  });

  return (
    <main className="mx-auto flex min-h-screen max-w-4xl flex-col gap-8 px-6 py-16">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-3xl font-semibold tracking-tight">
            Staff Transport
          </h1>
          <p className="text-muted-foreground">
            Signed in as {user?.name} · {user?.role}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={health.isSuccess ? "default" : "secondary"}>
            {health.isSuccess
              ? "API online"
              : health.isError
                ? "API offline"
                : "Checking API"}
          </Badge>
          <Button variant="outline" onClick={() => void logout()}>
            Sign out
          </Button>
        </div>
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
          <CardTitle>Getting started</CardTitle>
          <CardDescription>
            Authentication is in place. Next up: manager staff, driver, and
            vehicle management.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Seed a manager account with{" "}
          <code className="rounded bg-muted px-1.5 py-0.5">
            make seed-manager
          </code>{" "}
          and sign in.
        </CardContent>
      </Card>
    </main>
  );
}

export default function Home() {
  return (
    <RequireAuth>
      <Dashboard />
    </RequireAuth>
  );
}
