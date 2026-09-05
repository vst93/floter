import { useCallback, useRef, useState } from "react";

export type ActionOperation = { id: string; kind: string } | null;

type UseExtensionActionsOptions = {
  refresh: () => Promise<unknown>;
  onError: (error: unknown) => void;
  onComplete?: () => void;
};

export function useExtensionActions({ refresh, onError, onComplete }: UseExtensionActionsOptions) {
  const [busy, setBusyState] = useState<ActionOperation>(null);
  // The guard has to read the CURRENT operation, not the one captured when
  // runMutation was last memoized: two clicks in the same tick both saw a
  // stale `null` and ran concurrently. The ref is also why `busy` is gone from
  // the deps array — the callback no longer needs to be rebuilt to stay right.
  const busyRef = useRef<ActionOperation>(null);
  /** Single writer for busy state: keeps the ref guard and the rendered state
   * from drifting apart, including for callers that set it directly. */
  const setBusy = useCallback((operation: ActionOperation) => {
    busyRef.current = operation;
    setBusyState(operation);
  }, []);
  const runMutation = useCallback(async (id: string, kind: string, action: () => Promise<unknown>) => {
    if (busyRef.current) return false;
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
  }, [onComplete, onError, refresh, setBusy]);

  return { busy, busyRef, setBusy, runMutation };
}
