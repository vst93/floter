import { useEffect, useRef, useState } from "react";
import { AlertCircle, Play, RefreshCw, SquareTerminal, Trash2 } from "lucide-react";
import type { Translate } from "../i18n";
import type { BrokerSessionInfo } from "../App";

/** How long the kill button stays in its armed confirm state before the
 * timeout reverts it to the plain icon. Same rhythm as the extensions panel's
 * inline discard confirmation (75fff0b). */
const KILL_CONFIRM_TIMEOUT = 3000;

/** While this page is mounted and the window is visible, re-list sessions on
 * this cadence: they come and go out of band (new daemons, exits, attaches)
 * and the list would otherwise silently go stale. */
const SESSIONS_POLL_INTERVAL = 5000;

type SessionsPageProps = {
  t: Translate;
  sessions: BrokerSessionInfo[];
  loading: boolean;
  error: boolean;
  /** Id of the session an async kill/refresh round-trip is running against. */
  actionId: string | null;
  dateFormatter: Intl.DateTimeFormat;
  onResume: (session: BrokerSessionInfo) => void;
  onKill: (session: BrokerSessionInfo) => void;
  onRefresh: () => void;
};

/** The sessions settings page: one row per daemon terminal session. All state
 * lives in `App` and arrives through props; the only local state is the kill
 * button's armed confirmation. */
export function SessionsPage({
  t,
  sessions,
  loading,
  error,
  actionId,
  dateFormatter,
  onResume,
  onKill,
  onRefresh,
}: SessionsPageProps) {
  // Killing a live session may take down a PTY that still has work running,
  // and the trigger is a 14px icon — far too small a target for an
  // irreversible action. The first click arms the button: it expands in place
  // into a "terminate?" text button tinted with the danger color, and a
  // second click (or 3s of inactivity) decides.
  const [killArmedId, setKillArmedId] = useState<string | null>(null);
  const killArmTimer = useRef<number | null>(null);

  const disarmKill = () => {
    if (killArmTimer.current !== null) {
      window.clearTimeout(killArmTimer.current);
      killArmTimer.current = null;
    }
    setKillArmedId(null);
  };

  const armKill = (sessionId: string) => {
    if (killArmTimer.current !== null) window.clearTimeout(killArmTimer.current);
    setKillArmedId(sessionId);
    killArmTimer.current = window.setTimeout(() => {
      killArmTimer.current = null;
      setKillArmedId(null);
    }, KILL_CONFIRM_TIMEOUT);
  };

  // Keep the latest callback without re-arming the interval on every render
  // (refreshTerminalSessions is a fresh function each App render).
  const onRefreshRef = useRef(onRefresh);
  useEffect(() => {
    onRefreshRef.current = onRefresh;
  }, [onRefresh]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      if (document.visibilityState === "visible") onRefreshRef.current();
    }, SESSIONS_POLL_INTERVAL);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => () => {
    if (killArmTimer.current !== null) window.clearTimeout(killArmTimer.current);
  }, []);

  return (
    <section className="settings-section session-manager">
      <div className="settings-section__heading">
        <h2 className="settings-section__label">{t("terminal.sessions")}</h2>
        <button
          type="button"
          className="session-manager__icon-button"
          aria-label={t("terminal.sessionsRefresh")}
          title={t("terminal.sessionsRefresh")}
          disabled={loading}
          onClick={onRefresh}
        >
          <RefreshCw size={14} strokeWidth={1.9} aria-hidden="true" />
        </button>
      </div>

      {sessions.length === 0 ? (
        <div className="session-manager__empty" role={error ? "alert" : undefined}>
          {error ? <AlertCircle size={20} strokeWidth={1.6} aria-hidden="true" /> : <SquareTerminal size={20} strokeWidth={1.6} aria-hidden="true" />}
          <span>{loading ? t("terminal.sessionsLoading") : error ? t("terminal.sessionsError") : t("terminal.sessionsEmpty")}</span>
        </div>
      ) : (
        <div className="session-manager__list">
          {sessions.map((session) => {
            const busy = actionId === session.sessionId;
            const resumable = !session.exited && !session.attached;
            const state = session.exited
              ? t("terminal.sessionExited")
              : session.attached
                ? t("terminal.sessionAttached")
                : t("terminal.sessionDetached");
            const created = new Date(session.createdAt);
            return (
              <div
                key={session.sessionId}
                className={`session-manager__row${resumable ? " session-manager__row--resumable" : ""}`}
                onClick={resumable && actionId === null ? () => onResume(session) : undefined}
              >
                <span className="session-manager__marker" aria-hidden="true">
                  <SquareTerminal size={16} strokeWidth={1.8} />
                </span>
                <span className="session-manager__main">
                  <span className="session-manager__name">
                    {session.name || t("terminal.sessionTitle", { id: session.sessionId.slice(0, 8) })}
                  </span>
                  <span className="session-manager__cwd">{session.cwd || "~"}</span>
                  <span className="session-manager__meta">
                    <span className={session.exited ? "session-state session-state--exited" : "session-state"}>
                      {state}
                    </span>
                    <span>{session.size || `${session.width}x${session.height}`}</span>
                    {!Number.isNaN(created.getTime()) && <span>{dateFormatter.format(created)}</span>}
                  </span>
                </span>
                <span className="session-manager__actions">
                  <button
                    type="button"
                    className={`session-manager__icon-button${resumable ? " session-manager__icon-button--resumable" : ""}`}
                    aria-label={t("terminal.sessionResume")}
                    title={t("terminal.sessionResume")}
                    disabled={session.exited || busy || actionId !== null}
                    onClick={(event) => {
                      event.stopPropagation();
                      onResume(session);
                    }}
                  >
                    <Play size={14} strokeWidth={1.9} aria-hidden="true" />
                  </button>
                  {killArmedId === session.sessionId ? (
                    <button
                      type="button"
                      className="session-manager__confirm"
                      aria-label={t("terminal.sessionKillArm")}
                      disabled={actionId !== null}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={(event) => {
                        event.stopPropagation();
                        disarmKill();
                        onKill(session);
                      }}
                    >
                      <Trash2 size={13} strokeWidth={1.9} aria-hidden="true" />
                      {t("terminal.sessionKillArm")}
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="session-manager__icon-button session-manager__icon-button--danger"
                      aria-label={t("terminal.sessionKill")}
                      title={t("terminal.sessionKill")}
                      disabled={busy || actionId !== null}
                      onClick={(event) => {
                        event.stopPropagation();
                        armKill(session.sessionId);
                      }}
                    >
                      <Trash2 size={14} strokeWidth={1.9} aria-hidden="true" />
                    </button>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
