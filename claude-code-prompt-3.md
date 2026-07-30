# floter Multi-Monitor Position Fix (macOS)

You are working on **floter**, a Tauri v2 floating terminal/launcher app. Do NOT modify any code under `~/.hermes/`.

## Bug: Multi-monitor auto-select doesn't work on macOS

The user has multiple monitors on macOS. When they summon the panel with the hotkey, it appears on the wrong monitor (or always on the primary one) instead of the monitor the mouse cursor is on.

## Root Cause Analysis

In `src-tauri/src/lib.rs`, the flow for revealing the window is:

```rust
fn reveal_saved_mode(window: &WebviewWindow, state: &AppState) -> Result<(), String> {
    // ...
    let _ = move_to_default_position(window, width, state);  // sets position while HIDDEN
    reveal_window(window)?;  // THEN shows the window
    // ...
}
```

`move_to_default_position` → `default_position` → `focused_monitor` → `cursor_monitor` calls `window.cursor_position()` to find which monitor the mouse is on, then `window.set_position()` to move the window there.

**The problem**: On macOS, `set_position()` on a **hidden** (unmapped) window may not take effect. The macOS window server honors position changes only when the window is visible. When `show()` is subsequently called, the window appears at its old position (or the default position), not the one we just set.

The same issue exists in `show_terminal()` and `show_input()` which both call `resize_window()` (which calls `move_to_default_position`) before `reveal_window()`.

## Fix

**On macOS, move the `set_position` call to AFTER `show()` and `set_focus()`.** The window must be visible before its position can be changed.

### Approach

1. **Split position-setting from the reveal flow on macOS.** Instead of setting position before show, show the window first, then set its position.

2. **Modify `reveal_saved_mode`, `show_terminal`, and `show_input`** to use a macOS-specific order:

```rust
fn reveal_saved_mode(window: &WebviewWindow, state: &AppState) -> Result<(), String> {
    let terminal = state.terminal_mode.load(Ordering::SeqCst);
    let mode = if terminal { "terminal" } else { "collapsed" };
    let width = if terminal { TERMINAL_WINDOW_WIDTH } else { INPUT_WINDOW_WIDTH };

    #[cfg(target_os = "macos")]
    {
        // macOS: show first, then position. The window server ignores
        // set_position on an unmapped window.
        reveal_window(window)?;
        let _ = move_to_default_position(window, width, state);
    }

    #[cfg(not(target_os = "macos"))]
    {
        let _ = move_to_default_position(window, width, state);
        reveal_window(window)?;
    }

    state.window_visible.store(true, Ordering::SeqCst);
    let _ = window.emit("floter://revealed", mode);
    Ok(())
}
```

3. **Same pattern for `show_terminal` and `show_input`**: These call `resize_window` (which sets position) then `reveal_window`. On macOS, the position should be set AFTER reveal. 

For `show_terminal` and `show_input`, the `resize_window` call also sets the window size. The size change on a hidden window may work (or may not), but the position definitely needs to be after show. The simplest approach: in `resize_window`, skip the `move_to_default_position` call on macOS when the window is not yet visible, and do it after `reveal_window` instead.

Actually, a cleaner approach: **Extract the position-setting into a separate step that can be called after `reveal_window`**.

Here's the cleanest design:

- Add a helper `fn position_on_focused_monitor(window, width, state)` that just calls `move_to_default_position` — this is the position-only step.
- In `reveal_saved_mode`, `show_terminal`, and `show_input`: on macOS, call `reveal_window` first, then `position_on_focused_monitor`. On other platforms, keep the current order (position then reveal).

For `show_terminal` and `show_input`, the `resize_window` function currently does both size + position. Split it:
- `resize_window` should only set the **size** (and the anchor preservation logic).
- Position is set separately via `move_to_default_position` at the right point in the flow.

Actually, looking more carefully at `resize_window`:

```rust
fn resize_window(window, width, height, preserve_anchor, state) -> Result<(), String> {
    let previous_position = window.outer_position().ok();
    let previous_size = window.outer_size().ok();
    let scale_factor = window.scale_factor().unwrap_or(1.0);

    window.set_size(LogicalSize::new(width, height)).map_err(|e| e.to_string())?;

    if preserve_anchor {
        // re-center horizontally based on previous position
        if let (Some(position), Some(size)) = (previous_position, previous_size) {
            let next_width = (width * scale_factor).round() as i32;
            let next_x = position.x + (size.width as i32 - next_width) / 2;
            window.set_position(PhysicalPosition::new(next_x, position.y)).map_err(|e| e.to_string())?;
            return Ok(());
        }
    }

    move_to_default_position(window, width, state)
}
```

The `preserve_anchor` path sets a specific position based on the previous window position — this is for the case where the window is already visible and we're just resizing it. The `move_to_default_position` path is for when the window was hidden and needs to be re-positioned.

So the fix is simpler than I thought:

1. **In `resize_window`**: The `preserve_anchor` path is fine — it preserves the current position. The issue is only the `move_to_default_position` fallback (when `preserve_anchor` is false, i.e., window was hidden).

2. **In `reveal_saved_mode`**: Currently calls `move_to_default_position` before `reveal_window`. On macOS, swap the order.

3. **In `show_terminal` and `show_input`**: They call `resize_window` (which may call `move_to_default_position` inside) then `reveal_window`. On macOS, the position set inside `resize_window` won't take effect because the window is hidden. 

The cleanest fix:

**Option A (simplest)**: In all three functions (`reveal_saved_mode`, `show_terminal`, `show_input`), on macOS, call `reveal_window` first, then do the position setting. For `show_terminal` and `show_input`, this means:
- Call a version of `resize_window` that only sets size (not position) when window is hidden
- After `reveal_window`, call `move_to_default_position`

**Option B (cleanest)**: Make `resize_window` not call `move_to_default_position` at all. Instead, each caller explicitly positions the window. When the window is being revealed from hidden, the caller positions after reveal.

Go with **Option B**: 

1. Remove the `move_to_default_position` call from the end of `resize_window`. Replace it with `Ok(())` — if `preserve_anchor` fails, just leave the position as-is (the caller will handle positioning).

2. In `show_terminal` and `show_input`: after `reveal_window`, call `move_to_default_position`.

3. In `reveal_saved_mode`: on macOS, call `reveal_window` first, then `move_to_default_position`. On other platforms, keep the current order (position then reveal).

Wait, but on Linux/Wayland, setting position before show might be important because Wayland compositors may animate window appearance. Let me keep the platform split.

Actually, the simplest and most correct approach across ALL platforms:

**Always show first, then position.** This works on macOS (window server needs visible window), and it also works on Linux and Windows. The only downside is a potential brief flash of the window at its old position before it moves, but since `set_position` is called immediately after `show()`, this should be imperceptible (both happen in the same event loop tick).

BUT — there's a subtlety. `show()` on macOS triggers window server animations. If we set position right after, the animation might start from the old position. To avoid the flash, we can:
- Set `visible: false` in config (already done)
- On macOS, use `set_position` AFTER `show()` but BEFORE `set_focus()`
- The `show()` call maps the window; `set_position` right after moves it before the next frame is drawn

Let me go with this approach:

### Changes to make:

1. **In `resize_window`**: Remove the `move_to_default_position` fallback at the end. If `preserve_anchor` fails (no previous position), just return `Ok(())` — the caller handles positioning.

2. **In `reveal_window`**: Don't change — it stays as show + focus.

3. **In `reveal_saved_mode`**: Show first, then position:
```rust
fn reveal_saved_mode(window: &WebviewWindow, state: &AppState) -> Result<(), String> {
    let terminal = state.terminal_mode.load(Ordering::SeqCst);
    let mode = if terminal { "terminal" } else { "collapsed" };
    let width = if terminal { TERMINAL_WINDOW_WIDTH } else { INPUT_WINDOW_WIDTH };

    reveal_window(window)?;
    let _ = move_to_default_position(window, width, state);
    
    state.window_visible.store(true, Ordering::SeqCst);
    let _ = window.emit("floter://revealed", mode);
    Ok(())
}
```

4. **In `show_terminal`**: Set size, reveal, then position:
```rust
fn show_terminal(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    resize_window(&window, TERMINAL_WINDOW_WIDTH, TERMINAL_WINDOW_HEIGHT, preserve_anchor, &state)?;
    reveal_window(&window)?;
    if !preserve_anchor {
        let _ = move_to_default_position(&window, TERMINAL_WINDOW_WIDTH, &state);
    }
    state.terminal_mode.store(true, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}
```

5. **In `show_input`**: Same pattern as `show_terminal`.

6. **In `resize_window`**: Change the end from `move_to_default_position(window, width, state)` to just `Ok(())`.

### Also fix: `toggle_window_visibility`

The `toggle_window_visibility` function calls `reveal_saved_mode` when the window is hidden, so it's covered by the change to `reveal_saved_mode`.

### Additional macOS consideration

On macOS, there may be a flash where the window appears at its old position before `set_position` moves it. To minimize this, we can try setting `set_visible_on_all_workspaces(true)` and `set_always_on_top(true)` before `show()` (which is already done in `reveal_window`), and call `set_position` immediately after `show()` but before `set_focus()`. 

Actually, looking at `reveal_window` again:
```rust
fn reveal_window(window: &WebviewWindow) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let _ = window.app_handle().show();
        let _ = window.set_visible_on_all_workspaces(true);
        let _ = window.set_always_on_top(true);
        let _ = window.unminimize();
    }
    window.show().map_err(|e| e.to_string())?;
    window.set_focus().map_err(|e| e.to_string())?;
    // ...
}
```

The position should be set between `show()` and `set_focus()`. So we should restructure `reveal_window` to allow inserting a position call between show and focus, OR we should inline the show+position+focus sequence in the callers.

The cleanest approach: **Add an optional position parameter to `reveal_window`**, or split it into `show_window` and `focus_window`:

Actually, simplest: just restructure the callers to do show + position + focus explicitly, and remove `reveal_window`. But that would be a big refactor. 

Instead, let's just do: `reveal_window` (which does show + focus), then immediately `set_position`. The position change happens within the same event loop tick, so the user shouldn't see a flash. macOS coalesces window changes within a single runloop iteration.

### Summary of changes

All in `src-tauri/src/lib.rs`:

1. `resize_window`: Remove `move_to_default_position` fallback, return `Ok(())` instead
2. `reveal_saved_mode`: Swap order — `reveal_window` first, then `move_to_default_position`
3. `show_terminal`: After `reveal_window`, call `move_to_default_position` if not preserving anchor
4. `show_input`: Same as `show_terminal`

## Verification

```bash
cd src-tauri && cargo check --all-targets 2>&1
cd .. && npx tsc --noEmit 2>&1
```

Both must pass with zero errors.

## Code Style
- Keep existing code style: detailed doc comments explaining WHY
- All comments in English
- No new dependencies
- This is a cross-platform fix (show-then-position works on all platforms), so no `#[cfg]` guards needed
