import { useCallback, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { BrokerSessionInfo } from "../App";
import { beginRequest, isCurrentRequest } from "../request-generation";
import { useImmediateState } from "./useImmediateState";

type UseSessionManagementOptions = {
  onRefreshSuccess?: (sessions: BrokerSessionInfo[]) => void;
};

/**
 * R133 · polling hygiene. The sessions page re-lists every 5s and the daemon
 * almost always answers with the same sessions, but `invoke` hands back a fresh
 * array every time, so an unconditional `setSessions` is a new reference — and
 * therefore a re-render of `App` and the whole settings surface — 0.2 times a
 * second for no change at all. Compare the fields the list actually renders
 * (id, name, attach/exit state, exit code, creation time, grid, cwd) before
 * writing; identical content keeps the current array and skips the render.
 */
function sameSessions(a: BrokerSessionInfo[], b: BrokerSessionInfo[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    const current = a[i];
    const next = b[i];
    if (
      current.sessionId !== next.sessionId ||
      current.name !== next.name ||
      current.attached !== next.attached ||
      current.exited !== next.exited ||
      current.exitCode !== next.exitCode ||
      current.createdAt !== next.createdAt ||
      current.width !== next.width ||
      current.height !== next.height ||
      current.size !== next.size ||
      current.cwd !== next.cwd
    ) {
      return false;
    }
  }
  return true;
}

export function useSessionManagement(options: UseSessionManagementOptions = {}) {
  const [sessions, setSessions] = useState<BrokerSessionInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [actionId, setActionId, actionRef] = useImmediateState<string | null>(null);
  const refreshPending = useRef<Promise<void> | null>(null);

  const requestGeneration = useRef(0);
  const refreshTimer = useRef<number | null>(null);

  /**
   * R133 · `silent` is the polling path. The 5s poll must not touch `loading`
   * (the refresh button is `disabled={loading}`, so a non-silent poll flickers
   * it off every 5s) and must not touch `error`: a poll that fails while a
   * previous failure is on screen leaves that failure alone, and a poll that
   * fails with a clean screen raises nothing — a transient re-list is not news
   * the user asked for. The manual path (the refresh button, `scheduleRefresh`,
   * the kill round-trip) keeps the original read/write of `loading` and `error`,
   * including clearing a stale `error` on success and the empty list on failure.
   */
  const refreshSessions = useCallback((silent = false) => {
    if (refreshPending.current) return refreshPending.current;
    const generation = beginRequest(requestGeneration);
    if (!silent) {
      setLoading(true);
      setError(false);
    }
    const request = invoke<BrokerSessionInfo[]>("term_list_sessions")
      .then((sessions) => {
        if (!isCurrentRequest(requestGeneration, generation)) return;
        setSessions((current) => (sameSessions(current, sessions) ? current : sessions));
        if (!silent) setError(false);
        options.onRefreshSuccess?.(sessions);
      })
      .catch(() => {
        if (!isCurrentRequest(requestGeneration, generation)) return;
        if (silent) return;
        setSessions([]);
        setError(true);
      })
      .finally(() => {
        refreshPending.current = null;
        if (!silent && isCurrentRequest(requestGeneration, generation)) {
          setLoading(false);
        }
      });
    refreshPending.current = request;
    return request;
  }, [options.onRefreshSuccess]);

  const scheduleRefresh = useCallback(() => {
    if (refreshTimer.current !== null) {
      window.clearTimeout(refreshTimer.current);
    }
    refreshTimer.current = window.setTimeout(() => {
      refreshTimer.current = null;
      void refreshSessions();
    }, 500);
  }, [refreshSessions]);

  const killSession = useCallback(async (session: BrokerSessionInfo) => {
    if (actionRef.current) return;
    setActionId(session.sessionId);
    try {
      await invoke("term_kill_session", { sessionId: session.sessionId });
      await refreshPending.current;
      await refreshSessions();
    } catch {
      await refreshPending.current;
      await refreshSessions();
      setError(true);
    } finally {
      setActionId(null);
    }
  }, [actionId, refreshSessions]);

  return {
    sessions,
    loading,
    error,
    actionId,
    refreshSessions,
    scheduleRefresh,
    killSession,
  };
}
