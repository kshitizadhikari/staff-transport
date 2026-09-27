"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { ApiError } from "@/lib/api";
import { VEHICLE_STATUSES, vehicleApi } from "@/lib/directory";

export default function VehiclesPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const debouncedSearch = useDebouncedValue(search);

  const query = useQuery({
    queryKey: ["vehicles", { search: debouncedSearch, status }],
    queryFn: () =>
      vehicleApi.list({
        search: debouncedSearch || undefined,
        status: status || undefined,
        page_size: 50,
      }),
  });

  const deactivate = useMutation({
    mutationFn: (id: string) => vehicleApi.deactivate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["vehicles"] }),
  });

  function handleDeactivate(id: string, registration: string) {
    if (!window.confirm(`Deactivate vehicle ${registration}?`)) {
      return;
    }
    deactivate.mutate(id);
  }

  const items = query.data?.data ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Vehicles</h1>
          <p className="text-sm text-muted-foreground">
            {query.data ? `${query.data.pagination.total} vehicles` : "Fleet"}
          </p>
        </div>
        <Button render={<Link href="/vehicles/new" />}>New vehicle</Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Search registration or model…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className="max-w-xs"
        />
        <select
          value={status}
          onChange={(event) => setStatus(event.target.value)}
          className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
        >
          <option value="">All statuses</option>
          {VEHICLE_STATUSES.map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </select>
      </div>

      {deactivate.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {deactivate.error instanceof ApiError
            ? deactivate.error.message
            : "Unable to deactivate vehicle."}
        </p>
      ) : null}

      <div className="rounded-xl ring-1 ring-foreground/10">
        {query.isLoading ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState
            message={
              query.error instanceof ApiError
                ? query.error.message
                : "Unable to load vehicles."
            }
            onRetry={() => void query.refetch()}
          />
        ) : items.length === 0 ? (
          <EmptyState message="No vehicles match your filters." />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Registration</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Capacity</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((vehicle) => (
                <TableRow key={vehicle.id}>
                  <TableCell className="font-medium">
                    {vehicle.registration_number}
                  </TableCell>
                  <TableCell>{vehicle.model ?? "—"}</TableCell>
                  <TableCell>{vehicle.capacity}</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        vehicle.status === "inactive" ? "secondary" : "default"
                      }
                    >
                      {vehicle.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        render={<Link href={`/vehicles/${vehicle.id}`} />}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={
                          vehicle.status === "inactive" || deactivate.isPending
                        }
                        onClick={() =>
                          handleDeactivate(
                            vehicle.id,
                            vehicle.registration_number,
                          )
                        }
                      >
                        Deactivate
                      </Button>
                    </div>
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
