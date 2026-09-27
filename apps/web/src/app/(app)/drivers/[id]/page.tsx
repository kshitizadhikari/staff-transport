"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";

import { DriverForm } from "@/components/driver-form";
import { ErrorState, LoadingState } from "@/components/states";
import { Card, CardContent } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { driverApi, type DriverInput } from "@/lib/directory";

export default function EditDriverPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["drivers", id],
    queryFn: () => driverApi.get(id),
  });

  const update = useMutation({
    mutationFn: (input: DriverInput) => driverApi.update(id, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["drivers"] });
      router.push("/drivers");
    },
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
            : "Unable to load driver."
        }
        onRetry={() => void query.refetch()}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Edit driver</h1>
      <Card>
        <CardContent>
          <DriverForm
            mode="edit"
            driver={query.data}
            onSubmit={(input) => update.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
