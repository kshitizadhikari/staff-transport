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
import { DRIVER_STATUSES, driverApi } from "@/lib/directory";

export default function DriversPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const debouncedSearch = useDebouncedValue(search);

  const query = useQuery({
    queryKey: ["drivers", { search: debouncedSearch, status }],
    queryFn: () =>
      driverApi.list({
        search: debouncedSearch || undefined,
        status: status || undefined,
        page_size: 50,
      }),
  });

  const deactivate = useMutation({
    mutationFn: (id: string) => driverApi.deactivate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["drivers"] }),
  });

  function handleDeactivate(id: string, name: string) {
    if (!window.confirm(`Deactivate ${name}? Their account will be disabled.`)) {
      return;
    }
    deactivate.mutate(id);
  }

  const items = query.data?.data ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Drivers</h1>
          <p className="text-sm text-muted-foreground">
            {query.data ? `${query.data.pagination.total} drivers` : "Driver roster"}
          </p>
        </div>
        <Button render={<Link href="/drivers/new" />}>New driver</Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Search name, email, phone…"
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
          {DRIVER_STATUSES.map((value) => (
            <option key={value} value={value}>
              {value.replace("_", " ")}
            </option>
          ))}
        </select>
      </div>

      {deactivate.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {deactivate.error instanceof ApiError
            ? deactivate.error.message
            : "Unable to deactivate driver."}
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
                : "Unable to load drivers."
            }
            onRetry={() => void query.refetch()}
          />
        ) : items.length === 0 ? (
          <EmptyState message="No drivers match your filters." />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Phone</TableHead>
                <TableHead>License</TableHead>
                <TableHead>Expiry</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((driver) => (
                <TableRow key={driver.id}>
                  <TableCell className="font-medium">{driver.name}</TableCell>
                  <TableCell>{driver.email ?? "—"}</TableCell>
                  <TableCell>{driver.phone ?? "—"}</TableCell>
                  <TableCell>{driver.license_number ?? "—"}</TableCell>
                  <TableCell>{driver.license_expiry ?? "—"}</TableCell>
                  <TableCell>
                    <Badge
                      variant={driver.status === "inactive" ? "secondary" : "default"}
                    >
                      {driver.status.replace("_", " ")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        render={<Link href={`/drivers/${driver.id}`} />}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={driver.status === "inactive" || deactivate.isPending}
                        onClick={() => handleDeactivate(driver.id, driver.name)}
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
