// R43 · a launched application is its own process, not a tenant of Floter's.
//
// The user's report, verbatim: 「启动器启动其他应用后，好像会占用这个工具的进程，
// 导致托盘中的相关操作会变成被启动应用的。也就是说，被启动的应用应该以独立进程的
// 状态去运行，而不应该在我们这个应用的下面」.
//
// R41 answered the *blocking* half (`gio launch … .status()` froze the event-loop
// thread and the tray with it) and detached the child with `setsid`. R43 re-audits
// the claim the user repeated, and finds the half R41 left: `setsid` starts a new
// **session**, it does not move the process between **cgroups**. On a systemd
// session the launched app therefore stayed inside Floter's own scope, and
// Floter's scope cleanup could take it with it. R43 routes every launch through
// `process_launch::spawn_application`, which on Linux wraps the program in a
// transient `systemd-run --user --scope` unit: a new session *and* a new cgroup.
//
// This file is a source pin. The behavioural half lives in the Rust suite
// (`process_launch::tests` spawns `true`/`sleep` and reads `/proc/<pid>/stat` for
// the session id); a node test cannot spawn, so what it can do is keep every
// launch path wired to the one helper — which is the regression the report is
// really about.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

test("R43 · the launch helper detaches a session AND leaves the caller's cgroup", async () => {
  const launch = await read("src-tauri/src/process_launch.rs");
  // The session half (R41) is still there.
  assert.match(launch, /libc::setsid\(\)/, "the child leads its own session");
  assert.match(launch, /DETACHED_PROCESS \| CREATE_NEW_PROCESS_GROUP/, "…and on Windows it owns no console");
  assert.match(launch, /Stdio::null\(\)/, "and holds none of Floter's stdio");
  // The cgroup half (R43): a transient user scope, entered through `--scope`
  // (so the app inherits the display environment while getting its own cgroup).
  assert.match(launch, /systemd-run/, "Linux launches through systemd-run");
  assert.match(launch, /--user/, "the scope is a *user* scope");
  assert.match(launch, /--scope/, "--scope inherits the caller's environment");
  assert.match(launch, /--unit=/, "the transient unit is named");
  assert.match(launch, /floter-launch-/, "…and the name is Floter's");
  // A session that is not a systemd session falls back to the plain detached
  // spawn rather than failing to launch.
  assert.match(
    launch,
    /spawn_detached\(&fallback_program, &fallback_args\)/,
    "a refused scope falls back to the plain detached spawn",
  );
  // The one entry point every path calls.
  assert.match(launch, /pub\(crate\) fn spawn_application</, "one entry point per platform");
});

test("R43 · every app-launch path goes through the one helper", async () => {
  // Each of these used to (or could) spawn a user-facing application. They must
  // route through `process_launch`, never `Command::new(...).spawn()` directly.
  const paths = [
    "src-tauri/src/commands/apps/linux.rs",
    "src-tauri/src/commands/apps/macos.rs",
    "src-tauri/src/commands/actions.rs",
    "src-tauri/src/browser_data/mod.rs",
  ];
  for (const path of paths) {
    const source = await read(path);
    assert.match(source, /process_launch::spawn_(application|detached)/, `${path} must launch through process_launch`);
  }
  // The browser URL opener was the last blocking one: it used `.status()` on the
  // synchronous command thread, so opening a link froze the launcher *and* left
  // the browser in Floter's group. It is now a detached launch.
  const browser = await read("src-tauri/src/browser_data/mod.rs");
  assert.doesNotMatch(
    browser,
    /Command::new\("xdg-open"\)[\s\S]{0,120}\.status\(\)/,
    "the URL opener must not block on the browser",
  );
  assert.match(browser, /spawn_application\("xdg-open"/, "it launches detached");
  // Linux `.desktop` entries are launched from their own `Exec=` line (own
  // scope), with `gio` kept only for D-Bus-activated entries and as a fallback.
  const linux = await read("src-tauri/src/commands/apps/linux.rs");
  assert.match(linux, /dbus_activatable/, "the D-Bus-activatable flag is parsed");
  assert.match(linux, /spawn_application\(program, arguments\)/, "the Exec argv is the launch");
});
