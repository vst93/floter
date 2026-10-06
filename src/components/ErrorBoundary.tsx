// R113 · the root render backstop.
//
// R111's read-only survey (finding E) found the app mounted with no boundary:
// a render-time throw unmounts the whole React tree and leaves the window
// blank. The one edge the survey could reach is `TerminalCanvas`'s constructor
// when `getContext("2d")` comes back null (`terminal/render.ts`), which would
// take the launcher down with it. This is the smallest answer to that: catch
// the throw, print one text line, and offer a retry that re-renders the same
// subtree. No panel, no new chrome, no automatic reload, no `invoke`, no
// report — the fallback is the launcher's own feedback row, reused whole.
//
// The reset is React's documented one (`setState` clearing the error flag), so
// a transient failure that has since cleared renders normally again; a
// deterministic one simply throws into the same boundary, which is what a
// boundary is for.

import { Component, type ErrorInfo, type ReactNode } from "react";
import { RefreshCw } from "lucide-react";
import { createTranslator, type Language, type Translate } from "../i18n";

/** The fallback's language. `App.tsx` writes the settled language onto
 *  `documentElement.lang`, so reading it back here is the one source that is
 *  already correct when a subtree throws — and it needs no settings round trip,
 *  which the boundary deliberately does not make. Anything that is not Chinese
 *  falls back to English. */
const fallbackTranslator = (): Translate => {
  const tag = typeof document === "undefined" ? "" : document.documentElement.lang;
  const language: Language = tag.toLowerCase().startsWith("zh") ? "zh" : "en";
  return createTranslator(language);
};

type ErrorBoundaryProps = { children: ReactNode };
type ErrorBoundaryState = { failed: boolean };

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: unknown, info: ErrorInfo): void {
    // The only record the failure gets: the console. No report, no reload.
    console.error("floter: render failed", error, info);
  }

  private readonly retry = (): void => {
    // React's own reset pattern: clear the flag and render the children again.
    this.setState({ failed: false });
  };

  render(): ReactNode {
    if (!this.state.failed) return this.props.children;
    const t = fallbackTranslator();
    // The launcher's feedback row, reused whole: the fallback is the same text
    // line the launcher prints for a failed action, not a second surface.
    return (
      <div className="launcher-feedback launcher-feedback--warning" role="alert">
        <span>{t("render.failed")}</span>
        <button
          type="button"
          className="launcher-feedback__retry"
          aria-label={t("render.retry")}
          title={t("render.retry")}
          onClick={this.retry}
        >
          <RefreshCw size={13} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>
    );
  }
}
