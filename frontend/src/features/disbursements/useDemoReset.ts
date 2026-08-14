import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { resetDemo, workersQueryKey } from "@/features/disbursements/queries";

type DemoResetOptions = {
  onReset: () => void;
};

export function useDemoReset({ onReset }: DemoResetOptions) {
  const queryClient = useQueryClient();
  const [isOpen, setOpen] = useState(false);
  const mutation = useMutation({
    mutationFn: resetDemo,
    onSuccess: async () => {
      onReset();
      setOpen(false);
      await queryClient.invalidateQueries({ queryKey: workersQueryKey });
    },
  });

  function changeOpen(open: boolean) {
    setOpen(open);
    if (!open) {
      mutation.reset();
    }
  }

  return {
    changeOpen,
    confirmReset: () => mutation.mutate(),
    errorMessage: mutation.error?.message,
    isOpen,
    isResetting: mutation.isPending,
    open: () => setOpen(true),
  };
}
