import type { ReactNode } from "react";

/** SETTINGS-APPLE · the grouped-card language.
 *
 *  The reference is macOS System Settings (a Keyboard pane) plus the Mac App
 *  Store's list rows. Both share one structure, and it is the structure this
 *  module makes reusable:
 *
 *    group title (outside the card, with a one-line explanation under it)
 *    ┌──────────────────────────── card ────────────────────────────┐
 *    │ label                          control                       │  row
 *    │ ───────────────────────────── (1px, inset to the text) ────── │
 *    │ label            (grey sublabel)                 control      │  row
 *    └──────────────────────────────────────────────────────────────┘
 *
 *  Four primitives, no third-party UI kit:
 *
 *    * `SettingsCard`   — the flat grouped card (radius, fill, edge). It is a
 *      *plane*, never glass: HIG puts Liquid Glass on the functional layer
 *      (the shell), and a card that blurred would be a second material inside
 *      the one sheet the app allows.
 *    * `SettingsRow`    — label + optional sublabel + optional leading icon +
 *      a trailing control slot, with the inset separator drawn as its own
 *      element so "the rule stops short of the right edge" is a thing in the
 *      markup rather than a border on the card.
 *    * `SettingsAction` — the right-aligned blue text action ("Edit…",
 *      "Restore defaults"), the App Store's "See All" idiom. It spends the
 *      accent on *text*, never on a fill, so it is not an accent-budget face.
 *    * `SettingsEmpty` — PAGES-APPLY's contribution: the one empty region
 *      (glyph, state, grey hint) every list shares, so a page that has nothing
 *      to show cannot invent a second empty-state language.
 *
 *  The four are deliberately presentation-only: every page keeps owning its
 *  state, its handlers and its stored keys. `SettingsRow` takes an optional
 *  `onActivate` for the App Store's *tappable* row (the sessions list resumes
 *  on a row click); the row paints the hover wash and the inset focus ring for
 *  it, so the keyboard contract is stated once instead of per page.
 */

type SettingsCardProps = {
  /** The rows. Order is paint order — the last row paints no separator. */
  children: ReactNode;
  /** Names the row group for assistive tech (the visible title is the
   *  section heading above the card, which is not part of it). */
  label?: string;
  /** Extra class for a card that needs page-specific geometry. */
  className?: string;
};

/** The flat grouped card. See the module comment for why it must not blur. */
export function SettingsCard({ children, label, className }: SettingsCardProps) {
  return (
    <div
      className={`settings-section__card${className ? ` ${className}` : ""}`}
      role="group"
      aria-label={label}
    >
      {children}
    </div>
  );
}

type SettingsRowProps = {
  /** The row's title. Kept as a node so a page can bold a value inline. */
  label: ReactNode;
  /** The grey second line (HIG: the explanation lives in the row, not in a
   *  footer). */
  sublabel?: ReactNode;
  /** A leading glyph — the App Store row's app icon slot. Decorative. */
  icon?: ReactNode;
  /** The trailing control: a switch, a segmented picker, a select, a value
   *  plus an action, a slider. */
  control?: ReactNode;
  /** Put the control on its own line under the label at full width. Used for
   *  the two controls that are wider than a trailing slot can be: a range and
   *  a segmented picker. */
  stacked?: boolean;
  /** Extra class for a row with page-specific geometry. */
  className?: string;
  /** PAGES-APPLY: makes the whole row a hit target — the App Store's tappable
   *  list row (the sessions list resumes on a row click). Purely additive: a
   *  row without it is the same presentational div it always was. The
   *  keyboard contract is the row's own (role, one tab stop, Enter/Space),
   *  which is why it lives on the primitive instead of on each page. */
  onActivate?: () => void;
  /** While an async round-trip for this row is in flight: the row keeps its
   *  role but loses the tab stop and refuses activation (`aria-disabled`). */
  activateDisabled?: boolean;
};

/** One row of a `SettingsCard`: label left, control right, inset rule under. */
export function SettingsRow({
  label,
  sublabel,
  icon,
  control,
  stacked = false,
  className,
  onActivate,
  activateDisabled = false,
}: SettingsRowProps) {
  const classes = [
    "settings-row",
    stacked ? "settings-row--stacked" : "",
    className ?? "",
  ].filter(Boolean).join(" ");
  const interactive = onActivate !== undefined;
  const inert = interactive && activateDisabled;
  return (
    <div
      className={classes}
      role={interactive ? "button" : undefined}
      tabIndex={interactive && !inert ? 0 : undefined}
      aria-disabled={inert ? true : undefined}
      onClick={interactive && !inert ? onActivate : undefined}
      onKeyDown={
        interactive
          ? (event) => {
              if (event.target !== event.currentTarget || event.repeat || event.nativeEvent.isComposing) return;
              if (event.key !== "Enter" && event.key !== " ") return;
              event.preventDefault();
              if (!inert) onActivate();
            }
          : undefined
      }
    >
      {icon !== undefined && (
        <span className="settings-row__icon" aria-hidden="true">{icon}</span>
      )}
      <span className="settings-row__main">
        <span className="settings-row__label">{label}</span>
        {sublabel !== undefined && (
          <span className="settings-row__sublabel">{sublabel}</span>
        )}
      </span>
      {control !== undefined && (
        <span className="settings-row__control">{control}</span>
      )}
      {/* The separator is the row's own element, not a border on the row: a
          border would run the full width and would be the card's edge rather
          than a rule between two rows. CSS drops it on the last row. */}
      <span className="settings-row__divider" aria-hidden="true" />
    </div>
  );
}

type SettingsActionProps = {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  title?: string;
};

/** The blue text action: right-aligned, on the label's baseline, no fill.
 *  "Restore defaults" (in a group heading) and the deep-link copy are the
 *  call sites. It keeps the historical `settings-reset` class name — the
 *  shape tests, the reduce-motion list and the style census all name it. */
export function SettingsAction({
  children,
  onClick,
  disabled = false,
  title,
}: SettingsActionProps) {
  return (
    <button
      type="button"
      className="settings-reset"
      disabled={disabled}
      title={title}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

type SettingsEmptyProps = {
  /** The quiet glyph above the sentence (HIG `feedback.md`: the copy is the
   *  loudest thing in the empty region). Decorative. */
  icon?: ReactNode;
  /** The state in one line — never an explanation. */
  title: ReactNode;
  /** The why, or what changes the situation. */
  hint?: ReactNode;
  /** An empty state that reports a failure announces itself. */
  alert?: boolean;
};

/** PAGES-APPLY · the shared empty state.
 *
 *  One empty region for the whole app: a glyph, the state in one line, and the
 *  grey hint underneath with its 320px measure. It is the clipboard page's
 *  empty language (title in `--text-secondary` at weight 580, hint in
 *  `--text-tertiary`) lifted into a primitive so a second page cannot invent a
 *  third one. Presentation only, like the primitives above. */
export function SettingsEmpty({ icon, title, hint, alert = false }: SettingsEmptyProps) {
  return (
    <div className="settings-empty" role={alert ? "alert" : undefined}>
      {icon !== undefined && (
        <span className="settings-empty__icon" aria-hidden="true">{icon}</span>
      )}
      <span className="settings-empty__title">{title}</span>
      {hint !== undefined && (
        <span className="settings-empty__hint">{hint}</span>
      )}
    </div>
  );
}

type SettingsScaleProps = {
  /** The word at the left end of the scale (the minimum). */
  low: string;
  /** The word at the right end (the maximum). */
  high: string;
};

/** The two small grey words under a range — macOS's "Off — Fast" labels.
 *  They name the *ends* of the axis; the current value is the readout. */
export function SettingsScale({ low, high }: SettingsScaleProps) {
  return (
    <span className="settings-slider__scale">
      <span className="settings-slider__scale-end">{low}</span>
      <span className="settings-slider__scale-end">{high}</span>
    </span>
  );
}
