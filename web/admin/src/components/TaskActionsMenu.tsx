import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import type { TaskEntry } from "../types";
import type { AvailableActionsResponse } from "../api";
import {
  getAvailableActions,
  applyTaskTransition,
  overrideTaskStatus,
  renameTask,
  updateTaskTopic,
  updateTaskGoal,
  deleteTask,
} from "../api";
import { useToast } from "../hooks/useToast";
import ConfirmDialog from "./ConfirmDialog";
import PromptDialog from "./PromptDialog";
import styles from "./TaskActionsMenu.module.css";

// Transitions that require a confirmation dialog before firing
const DESTRUCTIVE_TRANSITIONS = new Set(["cancel", "start_over", "reimplement"]);

// Transitions that cause the backend to emit a gateway signal, which triggers
// the daemon to spawn a worker. See:
//   - config/taskfsm/events.go         — published request tokens
//   - config/taskfsm/gateway_signal.go:GatewaySignalTypeForEvent — canonical
//     signal-bearing lifecycle events
const SIGNAL_EMITTING_TRANSITIONS = new Set([
  "planner_finished",
  "implement_finished",
  "review_approved",
  "review_changes",
  "review_changes_requested",
  "verify_approved",
  "verify_failed",
]);

function transitionSpawnsWorker(event: string): boolean {
  return SIGNAL_EMITTING_TRANSITIONS.has(event.replace(/-/g, "_"));
}

export interface TaskActionsMenuProps {
  project: string;
  task: TaskEntry;
  variant?: "button" | "kebab";
  onChanged?: () => void | Promise<void>;
  onRenamed?: (newFilename: string) => void;
  onDeleted?: () => void;
}

type DialogState =
  | { kind: "none" }
  | { kind: "confirmTransition"; event: string; label: string }
  | { kind: "confirmOverride"; target: string; label: string }
  | { kind: "confirmDelete" }
  | { kind: "rename" }
  | { kind: "topic" }
  | { kind: "goal" };

interface PopoverPos {
  top: number;
  left: number;
}

export default function TaskActionsMenu({
  project,
  task,
  variant = "button",
  onChanged,
  onRenamed,
  onDeleted,
}: TaskActionsMenuProps) {
  const toast = useToast();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<PopoverPos>({ top: 0, left: 0 });
  const [actions, setActions] = useState<AvailableActionsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [focusedIdx, setFocusedIdx] = useState(-1);
  const [dialog, setDialog] = useState<DialogState>({ kind: "none" });
  const [busy, setBusy] = useState(false);
  const mutationLockRef = useRef(false);

  // Compute popover position relative to trigger
  const reposition = useCallback(() => {
    if (!triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const menuWidth = 220;
    let left = rect.left;
    // keep inside viewport
    if (left + menuWidth > window.innerWidth) {
      left = window.innerWidth - menuWidth - 8;
    }
    setPos({ top: rect.bottom + 4, left });
  }, []);

  // Open / fetch actions
  const handleOpen = useCallback(async () => {
    reposition();
    setOpen(true);
    setFocusedIdx(-1);
    setLoading(true);
    try {
      const data = await getAvailableActions(project, task.filename);
      setActions(data);
      // Move focus to the first menu item so arrow keys + enter work without a mouse.
      setFocusedIdx(0);
    } catch (err) {
      toast.show(`failed to load actions: ${String(err)}`, { kind: "error" });
      setOpen(false);
    } finally {
      setLoading(false);
    }
  }, [project, task.filename, reposition, toast]);

  // Refresh actions when task changes while menu is open
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    getAvailableActions(project, task.filename)
      .then((data) => {
        if (!cancelled) setActions(data);
      })
      .catch(() => {
        // silently ignore refresh errors
      });
    return () => { cancelled = true; };
  }, [open, project, task.filename, task.status]);

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
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Focus the popover itself when it opens so keyboard events (Escape, arrows)
  // are captured even before actions finish loading and the first item can be
  // focused.
  useEffect(() => {
    if (!open) return;
    popoverRef.current?.focus();
  }, [open]);

  // Focus the right item when focusedIdx changes
  useEffect(() => {
    if (!open || focusedIdx < 0) return;
    const el = popoverRef.current?.querySelectorAll<HTMLButtonElement>(
      "[data-menu-item]",
    )[focusedIdx];
    el?.focus();
  }, [open, focusedIdx]);

  // ---- mutation helpers -------------------------------------------------------

  async function handleTransition(event: string, label: string) {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      await applyTaskTransition(project, task.filename, event);
      toast.show(
        transitionSpawnsWorker(event)
          ? `transition '${label}' applied - daemon signal queued`
          : `transition '${label}' applied`,
      );
      setOpen(false);
      setDialog({ kind: "none" });
      await onChanged?.();
    } catch (err) {
      toast.show(`transition failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  async function handleOverride(target: string, label: string) {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      await overrideTaskStatus(project, task.filename, target);
      toast.show(`status overridden to '${label}'`);
      setOpen(false);
      setDialog({ kind: "none" });
      await onChanged?.();
    } catch (err) {
      toast.show(`override failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  async function handleRename(newFilename: string) {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      const updated = await renameTask(project, task.filename, newFilename);
      toast.show("task renamed");
      setOpen(false);
      setDialog({ kind: "none" });
      onRenamed?.(updated.filename);
      await onChanged?.();
    } catch (err) {
      toast.show(`rename failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  async function handleTopic(topic: string) {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      await updateTaskTopic(project, task.filename, topic);
      toast.show(topic ? "topic updated" : "topic cleared");
      setOpen(false);
      setDialog({ kind: "none" });
      await onChanged?.();
    } catch (err) {
      toast.show(`topic update failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  async function handleGoal(goal: string) {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      await updateTaskGoal(project, task.filename, goal);
      toast.show(goal ? "goal updated" : "goal cleared");
      setOpen(false);
      setDialog({ kind: "none" });
      await onChanged?.();
    } catch (err) {
      toast.show(`goal update failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  async function handleDelete() {
    if (mutationLockRef.current) return;
    mutationLockRef.current = true;
    setBusy(true);
    try {
      await deleteTask(project, task.filename);
      toast.show("task deleted");
      setOpen(false);
      setDialog({ kind: "none" });
      onDeleted?.();
    } catch (err) {
      toast.show(`delete failed: ${String(err)}`, { kind: "error" });
    } finally {
      mutationLockRef.current = false;
      setBusy(false);
    }
  }

  const menuItems = useMemo(() => {
    if (!actions) return [] as Array<{ key: string; label: string; action: () => void }>;

    const items: Array<{ key: string; label: string; action: () => void }> = [];

    for (const t of actions.transitions) {
      items.push({
        key: `transition-${t.event}`,
        label: t.label,
        action: () => {
          if (DESTRUCTIVE_TRANSITIONS.has(t.event)) {
            setDialog({ kind: "confirmTransition", event: t.event, label: t.label });
          } else {
            void handleTransition(t.event, t.label);
          }
        },
      });
    }

    for (const o of actions.overrides) {
      items.push({
        key: `override-${o.target}`,
        label: `override → ${o.label}`,
        action: () => {
          setDialog({ kind: "confirmOverride", target: o.target, label: o.label });
        },
      });
    }

    items.push(
      { key: "rename", label: "rename task", action: () => setDialog({ kind: "rename" }) },
      { key: "topic", label: "set topic", action: () => setDialog({ kind: "topic" }) },
      { key: "goal", label: "set goal", action: () => setDialog({ kind: "goal" }) },
      { key: "delete", label: "delete task", action: () => setDialog({ kind: "confirmDelete" }) },
    );

    return items;
  }, [actions, handleTransition]);

  // Keyboard nav inside the popover
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        setOpen(false);
        triggerRef.current?.focus();
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setFocusedIdx((i) => Math.min(i + 1, menuItems.length - 1));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setFocusedIdx((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === "Enter" && focusedIdx >= 0) {
        e.preventDefault();
        menuItems[focusedIdx]?.action();
        return;
      }
    },
    [menuItems, focusedIdx],
  );

  // ---- render -----------------------------------------------------------------

  const items = open ? menuItems : [];

  const transitionItems = items.filter((i) => i.key.startsWith("transition-"));
  const overrideItems = items.filter((i) => i.key.startsWith("override-"));
  const metaItems = items.filter((i) =>
    ["rename", "topic", "goal"].includes(i.key),
  );
  const deleteItem = items.find((i) => i.key === "delete");

  // Global flat index tracking for keyboard nav
  const allItems = [...transitionItems, ...overrideItems, ...metaItems, ...(deleteItem ? [deleteItem] : [])];

  function itemIndex(key: string): number {
    return allItems.findIndex((i) => i.key === key);
  }

  return (
    <>
      {/* Trigger */}
      <button
        ref={triggerRef}
        className={`${styles.trigger} ${variant === "kebab" ? styles.kebab : styles.buttonVariant}`}
        aria-haspopup="true"
        aria-expanded={open}
        onClick={() => (open ? setOpen(false) : void handleOpen())}
        disabled={busy}
      >
        {variant === "kebab" ? "⋮" : "actions"}
      </button>

      {/* Popover portal */}
      {open &&
        createPortal(
          <div
            ref={popoverRef}
            className={styles.popover}
            style={{ top: pos.top, left: pos.left }}
            role="menu"
            aria-label="task actions"
            tabIndex={-1}
            onKeyDown={handleKeyDown}
          >
            {loading && (
              <div className={styles.loading}>loading…</div>
            )}

            {!loading && (
              <>
                {/* Transitions */}
                {transitionItems.length > 0 && (
                  <section className={styles.section}>
                    <span className={styles.sectionLabel}>transitions</span>
                    {transitionItems.map((item) => {
                      const idx = itemIndex(item.key);
                      const isDestructive = DESTRUCTIVE_TRANSITIONS.has(
                        item.key.replace("transition-", ""),
                      );
                      return (
                        <button
                          key={item.key}
                          data-menu-item
                          className={`${styles.item} ${isDestructive ? styles.itemDestructive : ""}`}
                          role="menuitem"
                          tabIndex={focusedIdx === idx ? 0 : -1}
                          onClick={item.action}
                          onMouseEnter={() => setFocusedIdx(idx)}
                          disabled={busy}
                        >
                          {item.label}
                        </button>
                      );
                    })}
                  </section>
                )}

                {/* Overrides */}
                {overrideItems.length > 0 && (
                  <section className={styles.section}>
                    <span className={styles.sectionLabel}>overrides</span>
                    {overrideItems.map((item) => {
                      const idx = itemIndex(item.key);
                      return (
                        <button
                          key={item.key}
                          data-menu-item
                          className={`${styles.item} ${styles.itemOverride}`}
                          role="menuitem"
                          tabIndex={focusedIdx === idx ? 0 : -1}
                          onClick={item.action}
                          onMouseEnter={() => setFocusedIdx(idx)}
                          disabled={busy}
                        >
                          {item.label}
                        </button>
                      );
                    })}
                  </section>
                )}

                {/* Metadata */}
                <section className={styles.section}>
                  <span className={styles.sectionLabel}>metadata</span>
                  {metaItems.map((item) => {
                    const idx = itemIndex(item.key);
                    return (
                      <button
                        key={item.key}
                        data-menu-item
                        className={styles.item}
                        role="menuitem"
                        tabIndex={focusedIdx === idx ? 0 : -1}
                        onClick={item.action}
                        onMouseEnter={() => setFocusedIdx(idx)}
                        disabled={busy}
                      >
                        {item.label}
                      </button>
                    );
                  })}
                </section>

                {/* Destructive */}
                {deleteItem && (
                  <section className={`${styles.section} ${styles.sectionDestructive}`}>
                    <button
                      data-menu-item
                      className={`${styles.item} ${styles.itemDestructive}`}
                      role="menuitem"
                      tabIndex={focusedIdx === itemIndex("delete") ? 0 : -1}
                      onClick={deleteItem.action}
                      onMouseEnter={() => setFocusedIdx(itemIndex("delete"))}
                      disabled={busy}
                    >
                      {deleteItem.label}
                    </button>
                  </section>
                )}
              </>
            )}
          </div>,
          document.body,
        )}

      {/* Confirm: transition */}
      {dialog.kind === "confirmTransition" && (
        <ConfirmDialog
          open
          title={`apply transition: ${dialog.label}`}
          message={`are you sure you want to apply the '${dialog.label}' transition to this task?`}
          confirmLabel={dialog.label}
          destructive
          busy={busy}
          onConfirm={() => void handleTransition(dialog.event, dialog.label)}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}

      {/* Confirm: override */}
      {dialog.kind === "confirmOverride" && (
        <ConfirmDialog
          open
          title={`override status: ${dialog.label}`}
          message={`this will forcibly set the task status to '${dialog.label}'. continue?`}
          confirmLabel="override"
          destructive
          busy={busy}
          onConfirm={() => void handleOverride(dialog.target, dialog.label)}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}

      {/* Confirm: delete */}
      {dialog.kind === "confirmDelete" && (
        <ConfirmDialog
          open
          title="delete task"
          message={`permanently delete '${task.filename}'? this cannot be undone.`}
          confirmLabel="delete"
          destructive
          busy={busy}
          onConfirm={() => void handleDelete()}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}

      {/* Prompt: rename */}
      {dialog.kind === "rename" && (
        <PromptDialog
          open
          title="rename task"
          label="new filename"
          initialValue={task.filename}
          placeholder="task-filename.md"
          submitLabel="rename"
          busy={busy}
          onSubmit={(v) => void handleRename(v)}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}

      {/* Prompt: topic */}
      {dialog.kind === "topic" && (
        <PromptDialog
          open
          title="set topic"
          label="topic (empty to clear)"
          initialValue={task.topic ?? ""}
          placeholder="topic name"
          submitLabel="save"
          allowEmpty
          busy={busy}
          onSubmit={(v) => void handleTopic(v)}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}

      {/* Prompt: goal */}
      {dialog.kind === "goal" && (
        <PromptDialog
          open
          title="set goal"
          label="goal (empty to clear)"
          initialValue={task.goal ?? ""}
          placeholder="describe the goal…"
          multiline
          submitLabel="save"
          allowEmpty
          busy={busy}
          onSubmit={(v) => void handleGoal(v)}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
    </>
  );
}
