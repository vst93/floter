// R7-3b · The toolbar's overflow menu.
//
// G-24 was "the toolbar's four actions and two always-on hint lines push the
// actual list below the fold": every action had the same visual weight as the
// one the user actually came for, and the two captions explaining the rare
// half were permanently on screen. The fix is a frequency split — the
// high-frequency action keeps its button, the rare ones move into this menu,
// and their captions move in with them (a caption is only useful at the moment
// the user is choosing the action it describes).
//
// This is a menu, not a toolbar: it is a floating functional-layer pane, so it
// draws the same material as every other floater (`--glass-float` over
// `--elev-3`) and enters on the house spring. Like every control in the app it
// carries **no** backdrop-filter of its own — that is the performance red line
// (one sheet of glass per surface, see `glass-material.test.ts`).
//
// Keyboard contract (the part a `<details>` menu gets for free but a custom
// popup has to be given):
//
//   * the trigger opens with ↓/↑/Enter/Space and reports `aria-expanded`;
//   * ↓/↑ walk the enabled items and wrap, Home/End jump to the ends;
//   * Enter/Space activate the focused item (native button semantics);
//   * Escape closes and hands the keyboard back to the trigger — and it is
//     consumed here (`stopPropagation`) because the window-level dismiss table
//     in `useAppKeyboard` would otherwise read the same Escape as "close the
//     settings surface". Menus nest inside surfaces; the innermost open thing
//     owns Escape.
import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
} from "react";
import { MoreHorizontal } from "lucide-react";

export type OverflowMenuItem = {
  id: string;
  label: string;
  /** A caption shown under the label, inside the menu. Menus are where a
   *  description is read (the user is choosing right now); in the toolbar row
   *  the same sentence was permanently on screen for a rare action. */
  description?: string;
  icon?: ReactNode;
  disabled?: boolean;
  onSelect: () => void;
};

export function OverflowMenu({
  label,
  items,
  note,
  disabled,
  align = "end",
}: {
  /** Accessible name of the trigger (e.g. "More actions"). */
  label: string;
  items: OverflowMenuItem[];
  /** One sentence carried into the menu when a menu item's meaning depends on
   *  it (R7-3b moved the toolbar's standing captions in here). */
  note?: string;
  disabled?: boolean;
  align?: "start" | "end";
}) {
  const [open, setOpen] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const itemRefs = useRef(new Map<string, HTMLButtonElement>());
  const menuId = useId();
  const enabled = items.filter((item) => !item.disabled);

  const close = (restoreFocus: boolean) => {
    setOpen(false);
    setActiveId(null);
    if (restoreFocus) triggerRef.current?.focus({ preventScroll: true });
  };

  const focusItem = (id: string | undefined) => {
    if (!id) return;
    setActiveId(id);
    itemRefs.current.get(id)?.focus({ preventScroll: true });
  };

  const openAt = (id?: string) => {
    const first = id ?? enabled[0]?.id;
    if (!first) return;
    setOpen(true);
    // The item is not in the DOM until React has committed the open state.
    window.setTimeout(() => focusItem(first), 0);
    setActiveId(first);
  };

  const step = (from: string | null, delta: number) => {
    if (!enabled.length) return;
    const index = Math.max(0, enabled.findIndex((item) => item.id === from));
    const next = enabled[(index + delta + enabled.length) % enabled.length];
    focusItem(next.id);
  };

  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (event: MouseEvent) => {
      if (event.target instanceof Node && !rootRef.current?.contains(event.target)) close(false);
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [open]);

  const onTriggerKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (open && event.key === "Escape") {
      // The menu is open but focus has not landed in it yet (or sits on the
      // trigger): the menu must close here and the Esc must be consumed, or
      // it bubbles to the app dismiss table and takes the surface with it.
      event.preventDefault();
      event.stopPropagation();
      close(true);
      return;
    }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    event.preventDefault();
    openAt(event.key === "ArrowDown" ? enabled[0]?.id : enabled[enabled.length - 1]?.id);
  };

  const onMenuKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      // Consumed: the surface behind the menu must not also dismiss.
      event.preventDefault();
      event.stopPropagation();
      close(true);
      return;
    }
    if (event.key === "Tab") {
      // Tab leaves the menu (and returns the keyboard to the trigger so the
      // document's tab order continues from where it was).
      close(true);
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      step(activeId, 1);
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      step(activeId, -1);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      focusItem(event.key === "Home" ? enabled[0]?.id : enabled[enabled.length - 1]?.id);
    }
  };

  return (
    <div className="extensions-overflow" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="extensions-icon-button extensions-overflow__trigger"
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={disabled}
        onClick={() => (open ? close(false) : openAt())}
        onKeyDown={onTriggerKeyDown}
      >
        <MoreHorizontal size={17} strokeWidth={2} aria-hidden="true" />
      </button>
      {open && note && (
        <p className="extensions-overflow__note">{note}</p>
      )}
      {open && (
        <div
          id={menuId}
          className={`extensions-overflow__items extensions-overflow__items--${align}`}
          role="menu"
          aria-label={label}
          onKeyDown={onMenuKeyDown}
        >
          {items.map((item) => (
            <button
              key={item.id}
              ref={(node) => {
                if (node) itemRefs.current.set(item.id, node);
                else itemRefs.current.delete(item.id);
              }}
              type="button"
              role="menuitem"
              tabIndex={-1}
              className="extensions-overflow__item"
              data-overflow-item={item.id}
              disabled={item.disabled}
              onMouseEnter={() => setActiveId(item.id)}
              onClick={() => {
                close(true);
                item.onSelect();
              }}
            >
              {item.icon}
              <span className="extensions-overflow__text">
                <span className="extensions-overflow__label">{item.label}</span>
                {item.description && (
                  <span className="extensions-overflow__description">{item.description}</span>
                )}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
