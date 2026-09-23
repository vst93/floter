// R44 · drives the copy notice's state machine (`terminal/copy-notice.ts`)
// with real timers, and nothing else.
//
// The machine is pure, so this hook is only the clock: whenever the phase
// changes it arms one timeout for that phase's own delay and advances on
// expiry. A second `showCopyNotice` before that timeout lands replaces the
// state object, which re-runs the effect and therefore restarts the delay —
// that is the "consecutive copies do not flicker" behaviour, and it needs no
// second timer.

import { useCallback, useEffect, useState } from "react";
import {
  COPY_NOTICE_IDLE,
  copyNoticeAdvance,
  copyNoticeDelay,
  copyNoticeShow,
  type CopyNoticeState,
} from "../terminal/copy-notice";
import type { MessageKey } from "../i18n";

export function useCopyNotice() {
  const [copyNotice, setCopyNotice] = useState<CopyNoticeState>(COPY_NOTICE_IDLE);

  const showCopyNotice = useCallback((message: MessageKey) => {
    setCopyNotice((previous) => copyNoticeShow(previous, message));
  }, []);

  useEffect(() => {
    const delay = copyNoticeDelay(copyNotice);
    if (delay === null) return;
    const timer = window.setTimeout(() => {
      setCopyNotice((previous) => copyNoticeAdvance(previous));
    }, delay);
    return () => window.clearTimeout(timer);
  }, [copyNotice]);

  return { copyNotice, showCopyNotice };
}
