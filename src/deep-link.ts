// R7-9 · the frontend half of the `floter://` scheme.
//
// There is deliberately **no action table here**. The backend owns the
// allow-list (`src-tauri/src/deep_link.rs`), routes every URL, and decides
// what each action may carry; the frontend only receives the two outcomes it
// has to render:
//
//   * a validated `connect` request → open the manifest review dialog;
//   * a refusal → one toast.
//
// Re-declaring `["open", "connect"]` here would be the second allow-list this
// round exists to prevent, so this module holds event names, the wire shape
// and the reuse of the app's existing 30s failure deduper — nothing that could
// disagree with the router.
//
// The one user-visible string the *scheme itself* contributes lives in
// `src/i18n.ts` as `settings.deepLinkRejected`, and the backend sends that key
// rather than a sentence (the same "host owns the words" rule the plugin-page
// bridge follows).

import { createFailureDeduper, FAILURE_NOTIFY_DEDUP_MS } from "./plugin-pages.ts";

/** Emitted once a `connect` manifest has been validated and staged. */
export const DEEP_LINK_CONNECT_EVENT = "floter://deep-link-connect";

/** Emitted when a `floter://` link was refused and the user should be told. */
export const DEEP_LINK_REJECT_EVENT = "floter://deep-link-rejected";

/** The dictionary key the backend sends with a refusal. */
export const DEEP_LINK_REJECT_KEY = "settings.deepLinkRejected";

/** The example shown and copied on the About page. */
export const DEEP_LINK_EXAMPLE = "floter://connect?manifest=/path/to/tool.json";

/** What the backend hands over for a validated `connect`. Mirrors
 *  `deep_link::ConnectRequest`. */
export type DeepLinkConnectRequest = {
  manifestPath: string;
  extensionName: string;
  /** `local` or `https`. */
  source: string;
};

/**
 * How long a refusal stays quiet after it has been raised once.
 *
 * Reuses the plugin-page failure window (`FAILURE_NOTIFY_DEDUP_MS`, 30s)
 * rather than defining a second number: both are "an automatic, externally
 * triggered failure must not fill the stack", and two constants would drift.
 */
export const DEEP_LINK_REJECT_DEDUP_MS = FAILURE_NOTIFY_DEDUP_MS;

/**
 * The gate a refusal passes before it becomes a toast.
 *
 * The trigger is external and can repeat without anybody pressing anything —
 * a page that retries a broken link, a shell loop — so one bad link must not
 * produce one toast per attempt. It is the *same* deduper the clipboard page's
 * 2s poll uses, so "an automatic failure is coalesced per key" has one
 * implementation in the app.
 */
export const deepLinkRejectGate = createFailureDeduper(DEEP_LINK_REJECT_DEDUP_MS);
