"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";

import { ErrorState, LoadingState } from "@/components/states";
import { VehicleForm } from "@/components/vehicle-form";
import { Card, CardContent } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { vehicleApi, type VehicleInput } from "@/lib/directory";

export default function EditVehiclePage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["vehicles", id],
    queryFn: () => vehicleApi.get(id),
  });

  const update = useMutation({
    mutationFn: (input: VehicleInput) => vehicleApi.update(id, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["vehicles"] });
      router.push("/vehicles");
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
            : "Unable to load vehicle."
        }
        onRetry={() => void query.refetch()}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Edit vehicle</h1>
      <Card>
        <CardContent>
          <VehicleForm
            mode="edit"
            vehicle={query.data}
            onSubmit={(input) => update.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
