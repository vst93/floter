// R11 · the readable half of an unavailable runtime binding.
//
// The backend answers a binding failure with a stable key plus a JSON payload
// (`binding-changed:{"path":"/opt/homebrew/bin/php"}`). Before R11 the drawer
// glued the raw code and the backend's English sentence together, so a Homebrew
// `php` upgrade reached the user as
// `不可用: binding-changed Executable fingerprint changed at /opt/homebrew/bin/php`
// — a key prefix plus a sentence in the wrong language.
//
// This module is the only place the wording lives, so both dictionaries stay
// symmetric and neither the code nor the detail reaches the user untranslated.
// It also recognises the English prose an older build persisted in
// `brokenReason`, so a failure recorded before R11 still renders in the user's
// language instead of as a stale sentence.
//
// A detail this module does not recognise is returned as `null`: the caller
// keeps whatever it already rendered, which is strictly better than swallowing
// it. Pure and DOM-free so the node suite drives it directly (the same split
// `run-errors.ts` established).

import type { MessageKey, Translate } from "../i18n";

/** The binding failure codes the backend emits as keyed details. */
export const BINDING_DETAIL_CODES = ["binding-changed", "binding-missing"] as const;
export type BindingDetailCode = (typeof BINDING_DETAIL_CODES)[number];

export type BindingDetail = {
  code: BindingDetailCode;
  path: string;
};

const asString = (value: unknown): string => (typeof value === "string" ? value : "");

/**
 * Split `binding-changed:{"path":…}` into its code and path.
 *
 * Deliberately conservative: the code must be one of the binding codes and the
 * remainder must parse as a JSON object with a non-empty `path`. A plain
 * sentence, a keyed message from another subsystem (`run_…`), and a malformed
 * payload all return `null`.
 */
export const parseBindingDetail = (detail: string | null | undefined): BindingDetail | null => {
  if (!detail) return null;
  const trimmed = detail.trim();
  const separator = trimmed.indexOf(":");
  if (separator < 0) return null;
  const code = trimmed.slice(0, separator);
  if (!(BINDING_DETAIL_CODES as readonly string[]).includes(code)) return null;
  const body = trimmed.slice(separator + 1).trim();
  if (!body.startsWith("{")) return null;
  let payload: unknown;
  try {
    payload = JSON.parse(body);
  } catch {
    return null;
  }
  if (payload === null || typeof payload !== "object" || Array.isArray(payload)) return null;
  const path = asString((payload as { path?: unknown }).path);
  if (!path) return null;
  return { code: code as BindingDetailCode, path };
};

/**
 * The English sentences an older build wrote into `brokenReason`. Recognised so
 * a persisted pre-R11 failure is still localised; the keyed form is preferred
 * and this list exists only for the migration window.
 */
const LEGACY_DETAIL_PREFIXES: readonly { prefix: string; code: BindingDetailCode }[] = [
  { prefix: "Executable fingerprint changed at ", code: "binding-changed" },
  { prefix: "Executable is no longer available at ", code: "binding-missing" },
];

const legacyBindingDetail = (detail: string): BindingDetail | null => {
  for (const { prefix, code } of LEGACY_DETAIL_PREFIXES) {
    if (detail.startsWith(prefix)) {
      const path = detail.slice(prefix.length).trim();
      if (path) return { code, path };
    }
  }
  return null;
};

const messageKey = (code: BindingDetailCode): MessageKey =>
  code === "binding-changed"
    ? "settings.extensions.bindingChangedDetail"
    : "settings.extensions.bindingMissingDetail";

/**
 * The localised sentence for a binding-failure detail, or `null` when the
 * detail did not come from this path. Every message names the file, and the
 * changed case names the remedy ("click Re-check").
 */
export const bindingDetailMessage = (
  detail: string | null | undefined,
  t: Translate,
): string | null => {
  if (!detail) return null;
  const parsed = parseBindingDetail(detail) ?? legacyBindingDetail(detail.trim());
  if (!parsed) return null;
  return t(messageKey(parsed.code), { path: parsed.path });
};

/**
 * The one-line reason a failure row/drawer shows: the localised binding detail
 * when the detail came from this path, otherwise the translated code followed
 * by whatever prose the backend recorded. Never the raw key on its own, and
 * never the key glued to an untranslated sentence.
 */
export const failureReason = (
  code: string | null | undefined,
  detail: string | null | undefined,
  t: Translate,
): string => {
  const localised = bindingDetailMessage(detail, t);
  if (localised) return localised;
  const codeLabel = code
    ? t(`settings.extensions.errorCode.${code}` as MessageKey)
    : "";
  return [codeLabel, detail ?? ""].filter(Boolean).join(" · ");
};

/** The dictionary keys this module can produce, for the symmetry sweep. */
export const BINDING_DETAIL_MESSAGE_KEYS: MessageKey[] = [
  "settings.extensions.bindingChangedDetail",
  "settings.extensions.bindingMissingDetail",
];
