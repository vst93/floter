import { useCallback, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { BrokerSessionInfo } from "../App";
import { beginRequest, isCurrentRequest } from "../request-generation";
import { useImmediateState } from "./useImmediateState";

type UseSessionManagementOptions = {
  onRefreshSuccess?: (sessions: BrokerSessionInfo[]) => void;
};

export function useSessionManagement(options: UseSessionManagementOptions = {}) {
  const [sessions, setSessions] = useState<BrokerSessionInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [actionId, setActionId, actionRef] = useImmediateState<string | null>(null);
  const refreshPending = useRef<Promise<void> | null>(null);

  const requestGeneration = useRef(0);
  const refreshTimer = useRef<number | null>(null);

  const refreshSessions = useCallback(() => {
    if (refreshPending.current) return refreshPending.current;
    const generation = beginRequest(requestGeneration);
    setLoading(true);
    setError(false);
    const request = invoke<BrokerSessionInfo[]>("term_list_sessions")
      .then((sessions) => {
        if (!isCurrentRequest(requestGeneration, generation)) return;
        setSessions(sessions);
        setError(false);
        options.onRefreshSuccess?.(sessions);
      })
      .catch(() => {
        if (!isCurrentRequest(requestGeneration, generation)) return;
        setSessions([]);
        setError(true);
      })
      .finally(() => {
        refreshPending.current = null;
        if (isCurrentRequest(requestGeneration, generation)) {
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
