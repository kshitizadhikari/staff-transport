"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { DriverForm } from "@/components/driver-form";
import { Card, CardContent } from "@/components/ui/card";
import { driverApi, type DriverInput } from "@/lib/directory";

export default function NewDriverPage() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: (input: DriverInput) => driverApi.create(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["drivers"] });
      router.push("/drivers");
    },
  });

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">New driver</h1>
      <Card>
        <CardContent>
          <DriverForm
            mode="create"
            onSubmit={(input) => create.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
