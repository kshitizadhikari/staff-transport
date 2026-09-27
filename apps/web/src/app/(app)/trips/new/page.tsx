"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { TripForm } from "@/components/trip-form";
import { Card, CardContent } from "@/components/ui/card";
import { tripApi, type TripInput } from "@/lib/trips";

export default function NewTripPage() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: (input: TripInput) => tripApi.create(input),
    onSuccess: async (trip) => {
      await queryClient.invalidateQueries({ queryKey: ["trips"] });
      router.push(`/trips/${trip.id}`);
    },
  });

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">New trip</h1>
      <Card>
        <CardContent>
          <TripForm
            onSubmit={(input) => create.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
