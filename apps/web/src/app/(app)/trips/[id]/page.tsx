"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";

import { ErrorState, LoadingState } from "@/components/states";
import { TripEditForm } from "@/components/trip-edit-form";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ApiError } from "@/lib/api";
import { tripApi, type TripUpdateInput } from "@/lib/trips";

export default function TripDetailPage() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["trips", id],
    queryFn: () => tripApi.get(id),
  });

  const cancel = useMutation({
    mutationFn: () => tripApi.cancel(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["trips"] }),
  });

  const update = useMutation({
    mutationFn: (input: TripUpdateInput) => tripApi.update(id, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["trips"] }),
  });

  if (query.isLoading) {
    return <LoadingState />;
  }
  if (query.isError || !query.data) {
    return (
      <ErrorState
        message={
          query.error instanceof ApiError
            ? query.error.message
            : "Unable to load trip."
        }
        onRetry={() => void query.refetch()}
      />
    );
  }

  const trip = query.data;
  const editable = trip.status !== "completed" && trip.status !== "cancelled";
  const stops = trip.stops ?? [];
  const passengers = trip.passengers ?? [];

  function handleCancel() {
    if (!window.confirm("Cancel this trip? This cannot be undone.")) {
      return;
    }
    cancel.mutate();
  }

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <Link href="/trips" className="text-sm text-muted-foreground">
            ‹ Trips
          </Link>
          <h1 className="text-2xl font-semibold capitalize tracking-tight">
            {trip.type.replace(/_/g, " ")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {formatWhen(trip.scheduled_start_at)}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge
            variant={trip.status === "cancelled" ? "secondary" : "default"}>
            {trip.status.replace(/_/g, " ")}
          </Badge>
          {editable ? (
            <Button
              variant="destructive"
              disabled={cancel.isPending}
              onClick={handleCancel}>
              Cancel trip
            </Button>
          ) : null}
        </div>
      </div>

      {cancel.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {cancel.error instanceof ApiError
            ? cancel.error.message
            : "Unable to cancel trip."}
        </p>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Assignment</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <span className="text-muted-foreground">Driver: </span>
            {trip.driver_name ?? "Unassigned"}
          </div>
          <div>
            <span className="text-muted-foreground">Vehicle: </span>
            {trip.vehicle_registration ?? "Unassigned"}
          </div>
          {trip.notes ? (
            <div className="sm:col-span-2">
              <span className="text-muted-foreground">Notes: </span>
              {trip.notes}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Stops</CardTitle>
        </CardHeader>
        <CardContent>
          {stops.length === 0 ? (
            <p className="text-sm text-muted-foreground">No stops.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>#</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Address</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {stops.map((stop) => (
                  <TableRow key={stop.id}>
                    <TableCell>{stop.sequence + 1}</TableCell>
                    <TableCell className="capitalize">{stop.type}</TableCell>
                    <TableCell>{stop.address ?? "—"}</TableCell>
                    <TableCell>{stop.status}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Passengers</CardTitle>
        </CardHeader>
        <CardContent>
          {passengers.length === 0 ? (
            <p className="text-sm text-muted-foreground">No passengers.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {passengers.map((passenger) => (
                  <TableRow key={passenger.id}>
                    <TableCell>{passenger.name}</TableCell>
                    <TableCell className="capitalize">
                      {passenger.status.replace(/_/g, " ")}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {editable ? (
        <Card>
          <CardHeader>
            <CardTitle>Update trip</CardTitle>
          </CardHeader>
          <CardContent>
            <TripEditForm
              trip={trip}
              onSubmit={(input) =>
                update.mutateAsync(input).then(() => undefined)
              }
            />
          </CardContent>
        </Card>
      ) : null}
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
