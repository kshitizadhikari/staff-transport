"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { VehicleForm } from "@/components/vehicle-form";
import { Card, CardContent } from "@/components/ui/card";
import { vehicleApi, type VehicleInput } from "@/lib/directory";

export default function NewVehiclePage() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: (input: VehicleInput) => vehicleApi.create(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["vehicles"] });
      router.push("/vehicles");
    },
  });

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">New vehicle</h1>
      <Card>
        <CardContent>
          <VehicleForm
            mode="create"
            onSubmit={(input) => create.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
