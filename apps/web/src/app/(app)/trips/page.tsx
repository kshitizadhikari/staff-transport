"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { TRIP_STATUSES, tripApi } from "@/lib/trips";

export default function TripsPage() {
  const [status, setStatus] = useState("");
  const [date, setDate] = useState("");

  const query = useQuery({
    queryKey: ["trips", { status, date }],
    queryFn: () =>
      tripApi.list({
        status: status || undefined,
        date: date || undefined,
        page_size: 50,
      }),
  });

  const items = query.data?.data ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Trips</h1>
          <p className="text-sm text-muted-foreground">
            {query.data ? `${query.data.pagination.total} trips` : "Trip schedule"}
          </p>
        </div>
        <Button render={<Link href="/trips/new" />}>New trip</Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <select
          value={status}
          onChange={(event) => setStatus(event.target.value)}
          className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm">
          <option value="">All statuses</option>
          {TRIP_STATUSES.map((value) => (
            <option key={value} value={value}>
              {value.replace(/_/g, " ")}
            </option>
          ))}
        </select>
        <input
          type="date"
          value={date}
          onChange={(event) => setDate(event.target.value)}
          className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
        />
      </div>

      <div className="rounded-xl ring-1 ring-foreground/10">
        {query.isLoading ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState
            message={
              query.error instanceof ApiError
                ? query.error.message
                : "Unable to load trips."
            }
            onRetry={() => void query.refetch()}
          />
        ) : items.length === 0 ? (
          <EmptyState
            message="No trips match your filters."
            action={
              <Button render={<Link href="/trips/new" />} size="sm">
                Create trip
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Scheduled</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Driver</TableHead>
                <TableHead>Vehicle</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((trip) => (
                <TableRow key={trip.id}>
                  <TableCell>{formatWhen(trip.scheduled_start_at)}</TableCell>
                  <TableCell className="capitalize">
                    {trip.type.replace(/_/g, " ")}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        trip.status === "cancelled" ? "secondary" : "default"
                      }>
                      {trip.status.replace(/_/g, " ")}
                    </Badge>
                  </TableCell>
                  <TableCell>{trip.driver_name ?? "—"}</TableCell>
                  <TableCell>{trip.vehicle_registration ?? "—"}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      render={<Link href={`/trips/${trip.id}`} />}>
                      View
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  );
}

function formatWhen(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}
