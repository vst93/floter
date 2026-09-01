// Timed one-shot feedback banners: the launcher card's error line and the
// terminal panel's status line. Each message clears itself after 4.5s, and a
// newer message restarts its surface's timer.
//
// Extracted verbatim from `App.tsx`; the two surfaces were a word-for-word
// duplicated timer pair, so they now share this hook.

import { useEffect, useRef, useState } from "react";
import type { MessageKey } from "../i18n";

export function useTimedFeedback() {
  const [launcherFeedback, setLauncherFeedback] = useState<MessageKey | null>(null);
  const [terminalFeedback, setTerminalFeedback] = useState<MessageKey | null>(null);
  const launcherFeedbackTimer = useRef<number | null>(null);
  const terminalFeedbackTimer = useRef<number | null>(null);

  const showLauncherFeedback = (key: MessageKey, duration = 4500) => {
    setLauncherFeedback(key);
    if (launcherFeedbackTimer.current !== null) {
      window.clearTimeout(launcherFeedbackTimer.current);
    }
    launcherFeedbackTimer.current = window.setTimeout(() => {
      launcherFeedbackTimer.current = null;
      setLauncherFeedback(null);
    }, duration);
  };

  const showTerminalFeedback = (key: MessageKey) => {
    setTerminalFeedback(key);
    if (terminalFeedbackTimer.current !== null) {
      window.clearTimeout(terminalFeedbackTimer.current);
    }
    terminalFeedbackTimer.current = window.setTimeout(() => {
      terminalFeedbackTimer.current = null;
      setTerminalFeedback(null);
    }, 4500);
  };

  // Unmount cleanup: a pending timer must not fire into a gone component.
  useEffect(() => () => {
    if (launcherFeedbackTimer.current !== null) {
      window.clearTimeout(launcherFeedbackTimer.current);
    }
    if (terminalFeedbackTimer.current !== null) {
      window.clearTimeout(terminalFeedbackTimer.current);
    }
  }, []);

  return {
    launcherFeedback,
    terminalFeedback,
    setLauncherFeedback,
    setTerminalFeedback,
    showLauncherFeedback,
    showTerminalFeedback,
  };
}
