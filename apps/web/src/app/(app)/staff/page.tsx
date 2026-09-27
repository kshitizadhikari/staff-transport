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
import { staffApi } from "@/lib/directory";

type ActiveFilter = "all" | "true" | "false";

export default function StaffPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [active, setActive] = useState<ActiveFilter>("all");
  const debouncedSearch = useDebouncedValue(search);

  const query = useQuery({
    queryKey: ["staff", { search: debouncedSearch, active }],
    queryFn: () =>
      staffApi.list({
        search: debouncedSearch || undefined,
        active: active === "all" ? undefined : active === "true",
        page_size: 50,
      }),
  });

  const deactivate = useMutation({
    mutationFn: (id: string) => staffApi.deactivate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["staff"] }),
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
          <h1 className="text-2xl font-semibold tracking-tight">Staff</h1>
          <p className="text-sm text-muted-foreground">
            {query.data ? `${query.data.pagination.total} staff` : "Staff directory"}
          </p>
        </div>
        <Button render={<Link href="/staff/new" />}>New staff</Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Search name, email, phone…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className="max-w-xs"
        />
        <select
          value={active}
          onChange={(event) => setActive(event.target.value as ActiveFilter)}
          className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm"
        >
          <option value="all">All statuses</option>
          <option value="true">Active</option>
          <option value="false">Inactive</option>
        </select>
      </div>

      {deactivate.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {deactivate.error instanceof ApiError
            ? deactivate.error.message
            : "Unable to deactivate staff member."}
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
                : "Unable to load staff."
            }
            onRetry={() => void query.refetch()}
          />
        ) : items.length === 0 ? (
          <EmptyState
            message="No staff match your filters."
            action={
              <Button render={<Link href="/staff/new" />} size="sm">
                Add staff
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Phone</TableHead>
                <TableHead>Department</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((member) => (
                <TableRow key={member.id}>
                  <TableCell className="font-medium">{member.name}</TableCell>
                  <TableCell>{member.email ?? "—"}</TableCell>
                  <TableCell>{member.phone ?? "—"}</TableCell>
                  <TableCell>{member.department ?? "—"}</TableCell>
                  <TableCell>
                    <Badge variant={member.active ? "default" : "secondary"}>
                      {member.active ? "Active" : "Inactive"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        render={<Link href={`/staff/${member.id}`} />}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={!member.active || deactivate.isPending}
                        onClick={() => handleDeactivate(member.id, member.name)}
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
