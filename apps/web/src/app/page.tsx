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

type HealthResponse = { status: string };

const experiences = [
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
  const health = useQuery({
    queryKey: ["health"],
    queryFn: () => apiFetch<HealthResponse>("/health"),
    retry: false,
  });

  return (
    <main className="mx-auto flex min-h-screen max-w-4xl flex-col gap-8 px-6 py-16">
      <header className="flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-semibold tracking-tight">
            Staff Transport
          </h1>
          <Badge variant={health.isSuccess ? "default" : "secondary"}>
            {health.isSuccess
              ? "API online"
              : health.isError
                ? "API offline"
                : "Checking API"}
          </Badge>
        </div>
        <p className="text-muted-foreground">
          Manager-driven transport operations. This is the initial application
          shell.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-3">
        {experiences.map((experience) => (
          <Card key={experience.title}>
            <CardHeader>
              <CardTitle>{experience.title}</CardTitle>
              <CardDescription>{experience.description}</CardDescription>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Getting started</CardTitle>
          <CardDescription>
            Start local infrastructure and the API, then build features in the
            order documented in docs/DEVELOPMENT.md.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          <code className="rounded bg-muted px-1.5 py-0.5">make infra-up</code>{" "}
          then <code className="rounded bg-muted px-1.5 py-0.5">make backend-run</code>
        </CardContent>
      </Card>
    </main>
  );
}
