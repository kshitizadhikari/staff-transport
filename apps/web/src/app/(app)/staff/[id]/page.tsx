"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";

import { StaffForm } from "@/components/staff-form";
import { ErrorState, LoadingState } from "@/components/states";
import { Card, CardContent } from "@/components/ui/card";
import { ApiError } from "@/lib/api";
import { staffApi, type StaffInput } from "@/lib/directory";

export default function EditStaffPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["staff", id],
    queryFn: () => staffApi.get(id),
  });

  const update = useMutation({
    mutationFn: (input: StaffInput) => staffApi.update(id, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["staff"] });
      router.push("/staff");
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
            : "Unable to load staff member."
        }
        onRetry={() => void query.refetch()}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Edit staff</h1>
      <Card>
        <CardContent>
          <StaffForm
            mode="edit"
            staff={query.data}
            onSubmit={(input) => update.mutateAsync(input).then(() => undefined)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
