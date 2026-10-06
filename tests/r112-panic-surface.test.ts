// R112 · the sync-command panic surface, and the semantics that make it special.
//
// R111 swept the production panic surface read-only: ten sites, nine of them
// S5 "contract" panics where an upstream invariant guarantees the call cannot
// fail. The tenth was different in kind. `clipboard_history/mod.rs`'s
// `mutate_history` carried an `expect` on the just-filled cache — a *synchronous*
// Tauri command chain — and on Linux a panic on that path does not unwind, it
// aborts the process (R111 measured `exit 134`). This guard is the forward
// half of R112: the abort point must stay gone, and the reasoning must stay
// written down, because a future round "tidying" the `?` back into an `expect`
// would look harmless and cost the whole process.
//
// The banned token is assembled from parts on purpose. The scan below reads
// `mod.rs`, not this file, but spelling the complete literal here would make
// this guard a copy of the very thing it forbids — and would let a careless
// copy-paste self-poison. Same shape as `tests/r109-rusqlite-version.test.ts`
// and `tests/r110-deps-policy.test.ts`.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const SOURCE = "src-tauri/src/clipboard_history/mod.rs";
const NOTES = "docs/AGENT-NOTES.md";

/** The abort point R112 removed, assembled so this file never spells it out. */
const BANNED = 'expect("just ' + 'initialized")';

/** The soft replacement that carries the same invariant without an abort. */
const REPLACEMENT = 'ok_or("History cache unavailable")';

/** The pre-existing soft path, and the function that owns the removed call. */
const SOFT_PATH = 'map_err(|_| "History cache poisoned"';
const OWNER = "fn mutate_history";

/** The AGENT-NOTES section R112 pinned the semantics into. */
const SECTION = "Tauri 2 命令 panic 传播语义";

/**
 * The two consequences the notes must state, each assembled from parts so the
 * anchors read as tokens rather than as one long sentence a rewrite could
 * accidentally satisfy. A sync panic aborts the process; an async panic hangs
 * the command's promise forever.
 */
const SYNC_ABORT = "同步命令（79 个）panic ⇒ 进程 " + "abort";
const ASYNC_HANG = "异步命令（38 个）panic ⇒ 该命令 promise 永久" + "挂起";

/** The C trampoline the sync panic escapes through, and therefore cannot unwind. */
const C_TRAMPOLINE = 'unsafe extern "C" fn';

test("the sync command chain carries no abort point", async () => {
  const source = await read(SOURCE);

  assert.ok(
    !source.includes(BANNED),
    `${SOURCE} restored \`${BANNED}\` — a sync command must not carry an abort point`,
  );

  // Non-vacuous: the file is the live one, and the call was *replaced*, not
  // simply deleted along with its owner.
  assert.ok(
    source.includes(REPLACEMENT),
    `${SOURCE} lost \`${REPLACEMENT}\`; the removed panic has no successor`,
  );
  assert.ok(
    source.includes(OWNER),
    `${SOURCE} no longer defines \`${OWNER}\`; the scan is reading the wrong file`,
  );
});

test("the neighbouring soft path was not swept up in the change", async () => {
  const source = await read(SOURCE);

  // R112 touched exactly one line. The lock-poison path was already a soft
  // `Result` and must stay that way — "unifying" it into a panic would be a
  // regression, not a tidy-up.
  assert.ok(
    source.includes(SOFT_PATH),
    `${SOURCE} lost \`${SOFT_PATH}\`; R112's one-line change grew`,
  );
});

test("the notes pin the Tauri panic semantics", async () => {
  const notes = await read(NOTES);

  assert.ok(
    notes.includes(SECTION),
    `${NOTES} lost the "${SECTION}" section — the semantics are undocumented again`,
  );
  assert.ok(
    notes.includes(SYNC_ABORT),
    `${NOTES} must state that a sync command panic aborts the process`,
  );
  assert.ok(
    notes.includes(ASYNC_HANG),
    `${NOTES} must state that an async command panic hangs the promise`,
  );
  assert.ok(
    notes.includes(C_TRAMPOLINE),
    `${NOTES} must name the \`${C_TRAMPOLINE}\` trampoline the panic escapes through`,
  );
});

test("this guard assembles the banned token, it does not spell it out", async () => {
  const self = await readFile(new URL(import.meta.url), "utf8");

  assert.ok(
    !self.includes(BANNED),
    "the guard must build the banned token from parts, not write it as a literal",
  );
});
