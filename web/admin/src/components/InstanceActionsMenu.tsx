import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { InstanceEntry, InstanceAction } from "../types";
import styles from "./InstanceActionsMenu.module.css";

// ---- exported helpers (testable without DOM) --------------------------------

export interface InstanceMenuItem {
  action: InstanceAction;
  label: string;
  enabled: boolean;
  destructive: boolean;
}

/** Returns the ordered list of menu items with enabled/disabled state derived
 *  exclusively from instance.valid_actions — no status string inference. */
export function getMenuItems(instance: InstanceEntry): InstanceMenuItem[] {
  const allowed = new Set(instance.valid_actions ?? []);
  return [
    { action: "pause", label: "pause", enabled: allowed.has("pause"), destructive: false },
    { action: "resume", label: "resume", enabled: allowed.has("resume"), destructive: false },
    { action: "restart", label: "restart", enabled: allowed.has("restart"), destructive: false },
    { action: "kill", label: "kill", enabled: allowed.has("kill"), destructive: true },
  ];
}

/** Keyboard transition outcomes for the popover key handler. Declarative so
 *  it can be unit-tested without a DOM. */
export type MenuKeyOutcome =
  | { type: "close" }
  | { type: "move"; nextIdx: number }
  | { type: "none" };

/** Pure reducer for the popover-level keyboard handler. Covers the
 *  Esc/ArrowUp/ArrowDown transitions and is the single source of truth for
 *  focus-bounds math (clamps at 0 and itemCount-1). */
export function reduceMenuKey(
  key: string,
  currentIdx: number,
  itemCount: number,
): MenuKeyOutcome {
  if (key === "Escape") return { type: "close" };
  if (itemCount <= 0) return { type: "none" };
  if (key === "ArrowDown") {
    return { type: "move", nextIdx: Math.min(currentIdx + 1, itemCount - 1) };
  }
  if (key === "ArrowUp") {
    return { type: "move", nextIdx: Math.max(currentIdx - 1, 0) };
  }
  return { type: "none" };
}

/** Pure gate for Enter/click activation: a menu item should fire onAction
 *  only when it's enabled and the component is not busy. */
export function shouldFireAction(item: InstanceMenuItem, busy: boolean): boolean {
  return item.enabled && !busy;
}

// ---- component ---------------------------------------------------------------

export interface InstanceActionsMenuProps {
  instance: InstanceEntry;
  busy?: boolean;
  onAction: (action: InstanceAction) => void;
}

interface PopoverPos {
  top: number;
  left: number;
}

export default function InstanceActionsMenu({
  instance,
  busy = false,
  onAction,
}: InstanceActionsMenuProps) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<PopoverPos>({ top: 0, left: 0 });
  const [focusedIdx, setFocusedIdx] = useState(-1);

  const items = getMenuItems(instance);

  const reposition = useCallback(() => {
    if (!triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const menuWidth = 160;
    let left = rect.right - menuWidth;
    if (left < 4) left = 4;
    setPos({ top: rect.bottom + 4, left });
  }, []);

  const openMenu = useCallback(() => {
    reposition();
    setOpen(true);
    // Start focus on the first enabled item. If every item is disabled we
    // fall back to -1 so the popover itself keeps focus for Escape handling.
    const firstEnabledIdx = items.findIndex((it) => it.enabled);
    setFocusedIdx(firstEnabledIdx);
  }, [reposition, items]);

  const closeMenu = useCallback(() => {
    setOpen(false);
    setFocusedIdx(-1);
    triggerRef.current?.focus();
  }, []);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node) &&
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
        setFocusedIdx(-1);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Reposition on scroll / resize while open
  useEffect(() => {
    if (!open) return;
    const handler = () => reposition();
    window.addEventListener("scroll", handler, true);
    window.addEventListener("resize", handler);
    return () => {
      window.removeEventListener("scroll", handler, true);
      window.removeEventListener("resize", handler);
    };
  }, [open, reposition]);

  // Focus the popover itself when it first opens so Escape / arrows work
  // even before the focus moves to the first item.
  useEffect(() => {
    if (!open) return;
    popoverRef.current?.focus();
  }, [open]);

  // Move focus to the right item when focusedIdx changes
  useEffect(() => {
    if (!open || focusedIdx < 0) return;
    const el = popoverRef.current?.querySelectorAll<HTMLButtonElement>(
      "[data-menu-item]",
    )[focusedIdx];
    el?.focus();
  }, [open, focusedIdx]);

  const handlePopoverKeyDown = (e: React.KeyboardEvent) => {
    const outcome = reduceMenuKey(e.key, focusedIdx, items.length);
    if (outcome.type === "none") return;
    e.preventDefault();
    if (outcome.type === "close") {
      closeMenu();
      return;
    }
    setFocusedIdx(outcome.nextIdx);
  };

  const handleItemActivate = (item: InstanceMenuItem) => {
    if (!shouldFireAction(item, busy)) return;
    setOpen(false);
    setFocusedIdx(-1);
    onAction(item.action);
  };

  return (
    <>
      <button
        ref={triggerRef}
        className={styles.trigger}
        aria-label="instance actions"
        aria-haspopup="true"
        aria-expanded={open}
        disabled={busy}
        onClick={(e) => {
          e.stopPropagation();
          if (open) {
            closeMenu();
          } else {
            openMenu();
          }
        }}
        onKeyDown={(e) => {
          // Always stop propagation so row keydown handler (Enter/Space for
          // row selection) does not fire when the trigger is focused.
          e.stopPropagation();
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            if (open) {
              closeMenu();
            } else {
              openMenu();
            }
          }
        }}
      >
        ⋯
      </button>

      {open &&
        createPortal(
          <div
            ref={popoverRef}
            className={styles.popover}
            style={{ top: pos.top, left: pos.left }}
            role="menu"
            aria-label="instance actions"
            tabIndex={-1}
            onKeyDown={handlePopoverKeyDown}
          >
            {items.map((item, idx) => (
              <button
                key={item.action}
                data-menu-item
                role="menuitem"
                className={[
                  styles.item,
                  item.destructive ? styles.itemDestructive : "",
                  !item.enabled || busy ? styles.itemDisabled : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                disabled={!item.enabled || busy}
                tabIndex={-1}
                onClick={(e) => {
                  e.stopPropagation();
                  handleItemActivate(item);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleItemActivate(item);
                  }
                }}
                onFocus={() => setFocusedIdx(idx)}
              >
                {item.label}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </>
  );
}
