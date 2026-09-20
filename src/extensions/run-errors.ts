// R9-5 · the readable half of a failed run.
//
// The backend answers a failed run with a stable key and a JSON payload
// (`run_program_missing:{"path":"/opt/homebrew/bin/node"}`), never a sentence.
// This module turns that into a localised message, and is the *only* place the
// wording lives — so both dictionaries stay symmetric and a new backend key
// cannot reach the user as a raw `run_…:` string.
//
// A message this module does not recognise is returned unchanged: the caller
// keeps its ordinary toast, which is strictly better than swallowing it.
//
// Pure and DOM-free so the node suite drives it directly (the same split
// `run-routing.ts` and `run-params.ts` established).

import type { MessageKey, Translate } from "../i18n";

/** Every key the backend's `extensions::run_error` can emit. */
export type RunErrorKey =
  | "run_script_missing"
  | "run_program_missing"
  | "run_program_not_executable"
  | "run_interpreter_missing"
  | "run_spawn_failed"
  | "run_timeout";

/** The payload shapes the backend emits, per key. Only the fields that key
 *  actually carries are declared; an absent field degrades to a blank rather
 *  than rendering `undefined`. */
type RunErrorPayload = {
  path?: unknown;
  language?: unknown;
  names?: unknown;
  searched?: unknown;
  detail?: unknown;
  seconds?: unknown;
};

const asString = (value: unknown): string => (typeof value === "string" ? value : "");

const asStringList = (value: unknown): string[] =>
  Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];

/**
 * Split a keyed backend message into its key and payload.
 *
 * Deliberately conservative: the key must start with `run_` and the remainder
 * must parse as a JSON object. `run_param_required:note` (the parameter path's
 * key-plus-id shape) and `run_already_in_flight:local.a` are therefore *not*
 * claimed here — those are handled by their own mappers, and a plain prose
 * error is left alone.
 */
export const parseRunError = (
  message: string,
): { key: RunErrorKey; payload: RunErrorPayload } | null => {
  const trimmed = message.trim();
  const separator = trimmed.indexOf(":");
  if (separator < 0) return null;
  const key = trimmed.slice(0, separator);
  const body = trimmed.slice(separator + 1).trim();
  if (!body.startsWith("{")) return null;
  let payload: unknown;
  try {
    payload = JSON.parse(body);
  } catch {
    return null;
  }
  if (payload === null || typeof payload !== "object" || Array.isArray(payload)) return null;
  const known: RunErrorKey[] = [
    "run_script_missing",
    "run_program_missing",
    "run_program_not_executable",
    "run_interpreter_missing",
    "run_spawn_failed",
    "run_timeout",
  ];
  if (!known.includes(key as RunErrorKey)) return null;
  return { key: key as RunErrorKey, payload: payload as RunErrorPayload };
};

/**
 * The localised message for a failed run, or `null` when the message did not
 * come from this path (so the caller keeps whatever it already showed).
 *
 * Every branch names the fact that lets the user act: *which* file, *which*
 * interpreter, *where* it looked, or *how long* it waited.
 */
export const runErrorMessage = (message: string, t: Translate): string | null => {
  const parsed = parseRunError(message);
  if (!parsed) return null;
  const { key, payload } = parsed;
  switch (key) {
    case "run_script_missing":
      return t("settings.extensions.runErrorScriptMissing", { path: asString(payload.path) });
    case "run_program_missing":
      return t("settings.extensions.runErrorProgramMissing", { path: asString(payload.path) });
    case "run_program_not_executable":
      return t("settings.extensions.runErrorNotExecutable", { path: asString(payload.path) });
    case "run_interpreter_missing":
      return t("settings.extensions.runErrorInterpreterMissing", {
        language: asString(payload.language),
        names: asStringList(payload.names).join(" / "),
        searched: asStringList(payload.searched).join("\n"),
      });
    case "run_spawn_failed":
      return t("settings.extensions.runErrorSpawnFailed", {
        path: asString(payload.path),
        detail: asString(payload.detail),
      });
    case "run_timeout": {
      const seconds = typeof payload.seconds === "number" ? payload.seconds : 0;
      return t("settings.extensions.runErrorTimeout", { seconds });
    }
    default:
      return null;
  }
};

/** The dictionary keys this module can produce, for the symmetry sweep. */
export const RUN_ERROR_MESSAGE_KEYS: MessageKey[] = [
  "settings.extensions.runErrorScriptMissing",
  "settings.extensions.runErrorProgramMissing",
  "settings.extensions.runErrorNotExecutable",
  "settings.extensions.runErrorInterpreterMissing",
  "settings.extensions.runErrorSpawnFailed",
  "settings.extensions.runErrorTimeout",
];
