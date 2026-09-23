// Window-level key dispatch across the app's four surfaces — terminal,
// settings, plugin page and the collapsed launcher — plus the collapsed-mode
// recovery path that retypes keys into the query when the input has somehow
// lost the keyboard.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref,
// value and callback the handler touches, so the behaviour is unchanged.

import { useEffect, type Dispatch, type RefObject, type SetStateAction } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  encodeKey,
  isTerminalCompositionKey,
  shouldUseTerminalTextInput,
} from "../terminal/keys";
import type { Selection, TerminalCanvas } from "../terminal/render";
import {
  IS_MAC,
  matchesResultShortcut,
  matchesShortcut,
  type ShortcutMap,
} from "../shortcuts";
import { type SettingsPage } from "../settings-persistence";
import { isArrowKeyEditableTarget, nextSettingsPage } from "../settings-nav";
import {
  resolveDismissRule,
  surfaceYieldsToModal,
  type DismissRule,
} from "../surface-policy";
import type { ViewMode } from "../App";
import type { LauncherItem } from "../launcher/LauncherResults";
import { resultIndexForSlot } from "../launcher/result-budget";
import type { MessageKey } from "../i18n";

export function useAppKeyboard(options: {
  mode: ViewMode;
  shortcuts: ShortcutMap;
  recordingAction: string | null;
  launcherResults: LauncherItem[];
  query: string;
  inputRef: RefObject<HTMLInputElement | null>;
  selectionRef: RefObject<Selection | null>;
  dimsRef: RefObject<{ cols: number; rows: number }>;
  /** Whether the surface that currently owns the keyboard can take input. */
  surfaceReady: () => boolean;
  terminalTextInputRef: RefObject<HTMLTextAreaElement | null>;
  terminalInputTarget: () => string;
  activeRenderer: () => TerminalCanvas | null;
  activeSurfaceRef: RefObject<"main" | "pinned">;
  setActiveSurface: (surface: "main" | "pinned") => void;
  focusCollapsedInput: (delay?: number) => void;
  returnToInputMode: () => Promise<void>;
  openInTerminal: () => Promise<unknown>;
  togglePinnedTerminal: () => Promise<unknown>;
  copySelection: () => void;
  pasteClipboard: () => void;
  closeSettings: () => void;
  openSettings: (page?: SettingsPage) => void;
  settingsPage: SettingsPage;
  changeSettingsPage: (page: SettingsPage) => void;
  settingsSidebarButtons: RefObject<Map<SettingsPage, HTMLButtonElement>>;
  refreshTerminalSessions: () => Promise<void>;
  runLauncherItem: (item: LauncherItem | undefined) => void;
  handleLauncherKey: (event: KeyboardEvent) => void;
  /** R31 · the collapsed surface's mode-aware Esc / Cmd+W rule. Called before
   *  the dismiss table so the config overlay and a plugin mode can claim the
   *  press; returns whether it did. */
  onLauncherDismiss: (event: KeyboardEvent) => boolean;
  /** R32 · the empty-word way out of a plugin mode: a Backspace on an already
   *  empty field leaves the plugin. Called before the fallback's own Backspace
   *  edit; returns whether it consumed the press. */
  onPluginModeBackspace: (event: KeyboardEvent) => boolean;
  resultShortcutSlots: Array<number | null>;
  setQuery: Dispatch<SetStateAction<string>>;
  setHistory: Dispatch<SetStateAction<string[]>>;
  showLauncherFeedback: (key: MessageKey, duration?: number) => void;
  setHistoryIndex: Dispatch<SetStateAction<number>>;
  collapsedCardRef: RefObject<HTMLDivElement | null>;
}) {
  const {
    mode,
    shortcuts,
    recordingAction,
    launcherResults,
    inputRef,
    selectionRef,
    dimsRef,
    surfaceReady,
    terminalTextInputRef,
    terminalInputTarget,
    activeRenderer,
    activeSurfaceRef,
    setActiveSurface,
    focusCollapsedInput,
    returnToInputMode,
    openInTerminal,
    togglePinnedTerminal,
    copySelection,
    pasteClipboard,
    closeSettings,
    openSettings,
    settingsPage,
    changeSettingsPage,
    settingsSidebarButtons,
    refreshTerminalSessions,
    runLauncherItem,
    handleLauncherKey,
    onLauncherDismiss,
    onPluginModeBackspace,
    resultShortcutSlots,
    setQuery,
    setHistory,
    showLauncherFeedback,
    setHistoryIndex,
    collapsedCardRef,
  } = options;

  useEffect(() => {
    /**
     * Perform the action the dismiss table selected for this press. The
     * preventDefault / stopPropagation / keyup-re-arm side effects live here
     * once, instead of being re-typed per surface; the action itself still
     * calls exactly the callback the hand-written branch called before R7-2.
     */
    const runDismissAction = (rule: DismissRule, event: KeyboardEvent) => {
      event.preventDefault();
      if (rule.stopPropagation) event.stopPropagation();
      if (rule.reassertOnKeyUp) {
        // On macOS the panel can update its first responder once more when Cmd
        // is released. Reassert the input after the complete shortcut is up.
        const onShortcutRelease = (release: KeyboardEvent) => {
          if (release.metaKey || release.ctrlKey || release.altKey || release.shiftKey) return;
          window.removeEventListener("keyup", onShortcutRelease);
          focusCollapsedInput();
        };
        window.addEventListener("keyup", onShortcutRelease);
        window.setTimeout(() => {
          window.removeEventListener("keyup", onShortcutRelease);
        }, 1500);
      }
      switch (rule.action) {
        case "hide-window":
          invoke("hide_window");
          return;
        case "close-settings":
          closeSettings();
          return;
        case "return-to-input":
          returnToInputMode();
          return;
      }
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || isTerminalCompositionKey(event)) return;
      const target = event.target instanceof HTMLElement ? event.target : null;
      if (event.key === "Enter" && target?.closest("[data-destructive-confirm]")) {
        event.preventDefault();
        return;
      }
      if (event.repeat && ["Enter", " "].includes(event.key) && target?.closest("button")) {
        event.preventDefault();
        return;
      }
      const radio = target?.closest<HTMLButtonElement>('[role="radio"]');
      if (radio && ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"].includes(event.key)) {
        const radios = Array.from(radio.closest('[role="radiogroup"]')?.querySelectorAll<HTMLButtonElement>('button[role="radio"]:not(:disabled)') ?? []);
        const index = radios.indexOf(radio);
        const next = event.key === "Home" ? 0 : event.key === "End" ? radios.length - 1
          : (index + (["ArrowLeft", "ArrowUp"].includes(event.key) ? -1 : 1) + radios.length) % radios.length;
        event.preventDefault();
        radios[next]?.focus();
        radios[next]?.click();
        return;
      }
      // The recorder listens in the capture phase; this is only a safety net.
      if (recordingAction) return;

      if (mode === "terminal") {
        // While an IME owns the keyboard, even Enter and configured shortcuts
        // can be part of candidate selection. WebKit may report the confirming
        // key with keyCode 229 after clearing isComposing.
        if (isTerminalCompositionKey(event)) return;

        // App shortcuts first, everything else is forwarded to the shell.
        // Escape and Cmd/Ctrl+W deliberately have no rule in the terminal row
        // of the dismiss table: Escape belongs to the shell (or to the pinned
        // card), and the Cmd chord is the configurable new-command binding
        // handled right here.
        const dismiss = resolveDismissRule("terminal", event, shortcuts);
        if (dismiss) {
          runDismissAction(dismiss, event);
          return;
        }
        if (matchesShortcut(event, shortcuts.open_external_terminal)) {
          event.preventDefault();
          void openInTerminal();
          return;
        }
        // Pin / unpin / move the floating card. Only meaningful while a
        // terminal session view is open (the requirement's precondition).
        if (matchesShortcut(event, shortcuts.pin_terminal)) {
          event.preventDefault();
          void togglePinnedTerminal();
          return;
        }
        // While the card owns the keyboard, Escape hands it back to the main
        // surface instead of reaching the pinned session's shell.
        if (event.key === "Escape" && activeSurfaceRef.current === "pinned") {
          event.preventDefault();
          setActiveSurface("main");
          return;
        }
        // Copy only claims the combination when there is something to copy, so
        // a Ctrl+C binding still interrupts the foreground process otherwise.
        // `selectionRef` and `copySelection` are both main-view-only: the card
        // has no selection of its own, so while it owns the keyboard this must
        // not fire — it would copy text from the other surface, and swallow the
        // press the pinned shell was waiting for.
        if (
          activeSurfaceRef.current === "main" &&
          selectionRef.current &&
          matchesShortcut(event, shortcuts.copy_selection)
        ) {
          event.preventDefault();
          copySelection();
          return;
        }
        if (matchesShortcut(event, shortcuts.paste)) {
          event.preventDefault();
          pasteClipboard();
          return;
        }
        // macOS keeps swallowing every other Cmd combo: those are window-level
        // shortcuts, never shell input. Ctrl combos on Windows and Linux are
        // the shell's (Ctrl+C, Ctrl+D, Ctrl+L, ...) and fall through.
        if (IS_MAC && event.metaKey && !event.altKey) {
          return;
        }
        if (event.shiftKey && (event.key === "PageUp" || event.key === "PageDown")) {
          event.preventDefault();
          // A page is the active surface's own row count. `dimsRef` measures the
          // main grid, and the card is typically much shorter, so using it there
          // would scroll the pinned session past whole screens of output the
          // user never saw. Falls back to the main dims when the card's renderer
          // has not laid out yet.
          const lines = Math.max(1, activeRenderer()?.rows ?? dimsRef.current.rows);
          invoke("term_scroll", {
            id: terminalInputTarget(),
            delta: event.key === "PageUp" ? lines : -lines,
          });
          return;
        }
        // The shell receives keystrokes only while the terminal's own proxy
        // input holds the keyboard AND the PTY is ready. Anything else is
        // dropped here rather than forwarded: presses that arrive during a
        // launcher→terminal transition, or while focus sits on a header
        // button, would otherwise land at the prompt as phantom commands —
        // exactly the "command not found" reports for text the user typed in
        // the launcher and never meant for the shell.
        // Readiness is asked of the surface that owns the keyboard, not of the
        // main view: while the card is focused the main slot is empty by design.
        if (
          document.activeElement !== terminalTextInputRef.current ||
          !surfaceReady()
        ) {
          return;
        }
        if (
          event.target === terminalTextInputRef.current &&
          shouldUseTerminalTextInput(event)
        ) {
          return;
        }
        const renderer = activeRenderer();
        const encoded = renderer ? encodeKey(event, renderer.mode) : null;
        if (encoded) {
          event.preventDefault();
          invoke("term_input", { id: terminalInputTarget(), data: Array.from(encoded) });
        }
        return;
      }

      if (mode === "settings") {
        // A modal owns the keyboard while it is open: it closes itself on
        // Esc/Cmd+W, and this handler must not run the surface rules (or the
        // sidebar arrows) underneath it. The surface that behaves this way is
        // declared in `surface-policy.ts`, not hard-coded here.
        if (surfaceYieldsToModal(mode) && target?.closest('[aria-modal="true"]')) return;
        const dismiss = resolveDismissRule(mode, event, shortcuts);
        if (dismiss) {
          runDismissAction(dismiss, event);
          return;
        }
        // ↑/↓ cycle sidebar pages from anywhere in the settings panel — a
        // sidebar button, a plain button, or `<body>` when no control holds
        // focus. Editing and native select/range controls keep their arrows,
        // as does any open modal (handled above).
        if (
          (event.key === "ArrowDown" || event.key === "ArrowUp") &&
          !isArrowKeyEditableTarget(target)
        ) {
          event.preventDefault();
          const next = nextSettingsPage(
            settingsPage,
            event.key === "ArrowDown" ? "down" : "up",
          );
          settingsSidebarButtons.current.get(next)?.focus();
          changeSettingsPage(next);
          if (next === "sessions") void refreshTerminalSessions();
          return;
        }
        return;
      }

      // Collapsed-mode input handling.
      if (matchesShortcut(event, shortcuts.open_settings)) {
        event.preventDefault();
        openSettings();
        return;
      }

      // Ctrl+Shift+Backspace clears the typed-command history. Launcher-only:
      // the terminal owns the keyboard in its own mode and we must not steal a
      // key the shell is waiting for.
      if (
        event.ctrlKey &&
        event.shiftKey &&
        event.key === "Backspace"
      ) {
        event.preventDefault();
        setHistory([]);
        setHistoryIndex(-1);
        showLauncherFeedback("launcher.historyCleared");
        return;
      }

      const dismiss = resolveDismissRule("collapsed", event, shortcuts);
      // R31 · the config overlay and a plugin mode are the two levels above the
      // window. If neither claims the press, the table's own rule runs —
      // unchanged for the ordinary search page.
      if (onLauncherDismiss(event)) return;
      if (dismiss) {
        runDismissAction(dismiss, event);
        return;
      }

      const inputFocused = document.activeElement === inputRef.current;
      const resultNumber = inputFocused ? null : matchesResultShortcut(event, shortcuts.select_result);
      if (resultNumber !== null) {
        // R19/R36: one mapping, shared with the badges — `⌘0` is the fixed
        // clipboard row (see `resultIndexForSlot`).
        const resultIndex = resultIndexForSlot(resultShortcutSlots, resultNumber);
        if (resultIndex >= 0) {
          event.preventDefault();
          runLauncherItem(launcherResults[resultIndex]);
        }
        return;
      }
      if (inputFocused) return;

      // Tab deliberately moves from the query into the session and settings
      // controls. Once focus is inside the card, leave ordinary button
      // keyboard handling to the browser instead of treating it as accidental.
      const activeElement = document.activeElement;
      if (activeElement && collapsedCardRef.current?.contains(activeElement)) return;

      // Below here the input does not have the keyboard, which in this mode is
      // only intentional while focus is on one of the card's own controls.
      // Anything outside the card takes the field back and then does what it
      // would have done had the field never lost it.
      focusCollapsedInput();

      // Losing focus must never turn an unrelated Enter into an execution.
      if (event.key === "Enter" || isTerminalCompositionKey(event)) {
        event.preventDefault();
        return;
      }

      if (event.metaKey || event.ctrlKey || event.altKey) return;

      // R32 · an empty field's Backspace leaves a plugin mode rather than
      // deleting nothing. Before the ordinary edit so the two cannot disagree.
      if (onPluginModeBackspace(event)) return;

      if (event.key === "Backspace") {
        event.preventDefault();
        setQuery((current) => current.slice(0, -1));
        return;
      }
      // A printable key is typed into the query by hand: the field was not
      // focused when the press happened, so nothing else will insert it.
      if (event.key.length === 1) {
        event.preventDefault();
        setQuery((current) => `${current}${event.key}`);
        setHistoryIndex(-1);
        return;
      }

      // Enter, the arrows, Tab — the keys the field's own handler owns.
      handleLauncherKey(event);
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  });
}
