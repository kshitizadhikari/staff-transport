"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { StaffForm } from "@/components/staff-form";
import { Card, CardContent } from "@/components/ui/card";
import { staffApi, type StaffInput } from "@/lib/directory";

export default function NewStaffPage() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: (input: StaffInput) => staffApi.create(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["staff"] });
      router.push("/staff");
    },
  });

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">
        New staff member
      </h1>
      <Card>
        <CardContent>
          <StaffForm
            mode="create"
            onSubmit={(input) => create.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
