// R7-10c · the menu bar / tray icon switch, as values.
//
// The page (`GeneralPage.tsx`) is JSX around these two decisions, and the Rust
// side mirrors them (`desired_tray_visibility` in `src-tauri/src/lib.rs`). The
// switch is a *setting*, not a command: the frontend persists
// `show_menubar_icon` through the ordinary settings save and the backend applies
// it, so there is no second source of truth to keep in step.
//
// Keeping the two one-liners here rather than inline in the JSX is what makes
// the round's red line testable: an inverted switch (persisting `current`, or
// treating `false` as visible) fails a unit test instead of only being visible
// on a real menu bar.

/** The value the switch writes when the user clicks it: the opposite of what
 *  is stored. `true` means "show the icon", so turning it off writes `false`. */
export function toggleMenubarIcon(current: boolean): boolean {
  return !current;
}

/** The visibility contract the setting encodes — the frontend's copy of the
 *  Rust `desired_tray_visibility`. The status item is shown exactly when the
 *  setting is on; nothing else in the app may invert or reinterpret it. */
export function menubarIconVisible(showMenubarIcon: boolean): boolean {
  return showMenubarIcon;
}

/** The `role="switch"` state the row renders for a stored value: the class
 *  suffix and the `aria-checked` attribute are both read off this, so a click
 *  can never light the track while telling assistive tech it is off. */
export function menubarIconSwitchState(showMenubarIcon: boolean): {
  active: boolean;
  ariaChecked: boolean;
} {
  const on = menubarIconVisible(showMenubarIcon);
  return { active: on, ariaChecked: on };
}
