import { useCallback, useState } from "react";

export type ActionOperation = { id: string; kind: string } | null;

type UseExtensionActionsOptions = {
  refresh: () => Promise<unknown>;
  onError: (error: unknown) => void;
  onComplete?: () => void;
};

export function useExtensionActions({ refresh, onError, onComplete }: UseExtensionActionsOptions) {
  const [busy, setBusy] = useState<ActionOperation>(null);
  const runMutation = useCallback(async (id: string, kind: string, action: () => Promise<unknown>) => {
    if (busy) return false;
    setBusy({ id, kind });
    try {
      const result = await action();
      if (result === false) return false;
      await refresh();
      onComplete?.();
      return true;
    } catch (error) {
      onError(error);
      return false;
    } finally {
      setBusy(null);
    }
  }, [busy, onComplete, onError, refresh]);

  return { busy, setBusy, runMutation };
}
