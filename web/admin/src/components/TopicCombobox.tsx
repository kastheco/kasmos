import { useEffect, useRef, useState } from "react";
import type { TopicEntry } from "../types";
import styles from "./TopicCombobox.module.css";

export type TopicSelection = { value: string; isNew: boolean };

interface TopicComboboxProps {
  topics: TopicEntry[];
  value: string;
  onChange(selection: TopicSelection): void;
  disabled?: boolean;
  id?: string;
  placeholder?: string;
}

/**
 * A combobox for picking an existing topic or declaring a new one.
 *
 * Keyboard model:
 *   ArrowDown / ArrowUp  — move highlight through the list
 *   Enter                — commit the highlighted row
 *   Escape               — close the list without changing the typed value
 *   any text key         — keeps typing into the input normally
 *
 * onChange is called on every input change (so the dialog can always read
 * the current selection without the user having to press Enter).
 */
export default function TopicCombobox({
  topics,
  value,
  onChange,
  disabled = false,
  id,
  placeholder = "select or create a topic",
}: TopicComboboxProps) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState(value);
  const [highlightIdx, setHighlightIdx] = useState(-1);

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Keep internal typed state in sync when the controlled value changes
  // from outside (e.g. reset).
  useEffect(() => {
    setTyped(value);
  }, [value]);

  // Build the filtered list of existing topic rows
  const filteredTopics = typed.trim()
    ? topics.filter((t) =>
        t.name.toLowerCase().includes(typed.trim().toLowerCase()),
      )
    : topics;

  // Show "create" row when there is typed text and no exact match
  const hasExactMatch = topics.some(
    (t) => t.name.toLowerCase() === typed.trim().toLowerCase(),
  );
  const showCreateRow = typed.trim() !== "" && !hasExactMatch;

  // All rows in display order: existing filtered topics, then optional create row
  const rowCount = filteredTopics.length + (showCreateRow ? 1 : 0);
  const createRowIdx = showCreateRow ? filteredTopics.length : -1;

  // Clamp highlight when the list shrinks
  useEffect(() => {
    if (highlightIdx >= rowCount) {
      setHighlightIdx(rowCount > 0 ? rowCount - 1 : -1);
    }
  }, [rowCount, highlightIdx]);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Scroll highlighted item into view
  useEffect(() => {
    if (!open || highlightIdx < 0 || !listRef.current) return;
    const item = listRef.current.children[highlightIdx] as HTMLElement | undefined;
    item?.scrollIntoView?.({ block: "nearest" });
  }, [open, highlightIdx]);

  function emitChange(rawTyped: string) {
    const trimmed = rawTyped.trim();
    if (!trimmed) {
      onChange({ value: "", isNew: false });
      return;
    }
    // Canonicalize case-insensitive exact matches: typing "Frontend" against
    // an existing "frontend" must emit "frontend", otherwise the backend will
    // auto-create a second topic key.
    const match = topics.find(
      (t) => t.name.toLowerCase() === trimmed.toLowerCase(),
    );
    if (match) {
      onChange({ value: match.name, isNew: false });
      return;
    }
    onChange({ value: rawTyped, isNew: true });
  }

  function commitRow(idx: number) {
    if (idx === createRowIdx) {
      // "create new" row
      onChange({ value: typed.trim(), isNew: true });
      setTyped(typed.trim());
    } else {
      const topic = filteredTopics[idx];
      if (!topic) return;
      onChange({ value: topic.name, isNew: false });
      setTyped(topic.name);
    }
    setOpen(false);
    setHighlightIdx(-1);
  }

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const newVal = e.target.value;
    setTyped(newVal);
    setOpen(true);
    setHighlightIdx(-1);
    emitChange(newVal);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open && e.key !== "Escape") {
      if (e.key === "ArrowDown" || e.key === "ArrowUp" || e.key === "Enter") {
        setOpen(true);
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlightIdx(0);
      }
      return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setHighlightIdx((i) => Math.min(i + 1, rowCount - 1));
        break;

      case "ArrowUp":
        e.preventDefault();
        setHighlightIdx((i) => Math.max(i - 1, 0));
        break;

      case "Enter":
        if (highlightIdx >= 0) {
          e.preventDefault();
          commitRow(highlightIdx);
        }
        break;

      case "Escape":
        // Only swallow Escape when the listbox is actually open. Otherwise
        // let it bubble so a containing dialog's Escape handler can close
        // the dialog. stopPropagation prevents the document-level dialog
        // listener in NewTaskDialog from also firing for this Escape.
        if (open) {
          e.preventDefault();
          e.stopPropagation();
          setOpen(false);
          setHighlightIdx(-1);
        }
        break;

      default:
        break;
    }
  }

  const listboxId = id ? `${id}-listbox` : "topic-listbox";

  return (
    <div ref={containerRef} className={styles.container}>
      <input
        ref={inputRef}
        id={id}
        type="text"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        aria-controls={listboxId}
        aria-activedescendant={
          open && highlightIdx >= 0
            ? `${listboxId}-option-${highlightIdx}`
            : undefined
        }
        className={styles.input}
        value={typed}
        onChange={handleInputChange}
        onFocus={() => setOpen(true)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete="off"
      />

      {open && rowCount > 0 && (
        <ul
          ref={listRef}
          id={listboxId}
          role="listbox"
          className={styles.listbox}
          aria-label="topics"
        >
          {filteredTopics.map((topic, idx) => (
            <li
              key={topic.name}
              id={`${listboxId}-option-${idx}`}
              role="option"
              aria-selected={highlightIdx === idx}
              className={`${styles.option} ${highlightIdx === idx ? styles.highlighted : ""}`}
              // Use onMouseDown so the selection fires before the input loses focus
              onMouseDown={(e) => {
                e.preventDefault();
                commitRow(idx);
              }}
            >
              {topic.name}
            </li>
          ))}

          {showCreateRow && (
            <li
              id={`${listboxId}-option-${createRowIdx}`}
              role="option"
              aria-selected={highlightIdx === createRowIdx}
              className={`${styles.option} ${styles.createRow} ${highlightIdx === createRowIdx ? styles.highlighted : ""}`}
              onMouseDown={(e) => {
                e.preventDefault();
                commitRow(createRowIdx);
              }}
            >
              create &ldquo;{typed.trim()}&rdquo;
            </li>
          )}
        </ul>
      )}
    </div>
  );
}
