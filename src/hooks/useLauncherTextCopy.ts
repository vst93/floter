// R63 · the 直出 (direct output) text surface's single copy chokepoint.
//
// R44 gave the terminal one function every copy path goes through
// (`useTerminalView`'s `copySelection`), reporting its outcome through the
// shared phase notice. The launcher's text surface needs the same contract —
// one write, one report — and it deliberately *shares the notice vocabulary*
// with the terminal so 「选中即复制」 reads identically on both surfaces (the
// user asked for "the same experience language", not a second set of words).
//
// The clipboard write takes the Rust-backed `clipboard_write_text` path first,
// exactly as the launcher's existing copy actions do: WebKitGTK ships without
// `navigator.clipboard`, so the backend is the authority.

import { useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";

import type { MessageKey } from "../i18n";

export function useLauncherTextCopy(showCopyNotice: (message: MessageKey) => void) {
  /** Put a finished text selection on the clipboard and say so.
   *
   *  The success notice fires only after the write resolved (the R44 rule), so
   *  "Copied" is never shown for a write that did not land. A blank string is
   *  refused here too, so the guard holds whatever the caller's own rule was. */
  const copySelection = useCallback(
    async (text: string) => {
      if (text.trim().length === 0) return;
      try {
        await invoke("clipboard_write_text", { text });
      } catch {
        showCopyNotice("terminal.copyNotice.failed");
        return;
      }
      showCopyNotice("terminal.copyNotice.copied");
    },
    [showCopyNotice],
  );

  return { copySelection };
}
