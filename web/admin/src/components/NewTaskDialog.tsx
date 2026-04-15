import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { TopicEntry, TaskEntry } from "../types";
import TopicCombobox from "./TopicCombobox";
import {
  deriveFilenameFromDescription,
  sanitizeFilenameInput,
  branchFromFilename,
  renderTaskStub,
  deriveTaskTitle,
} from "../lib/taskCreation";
import {
  createTask,
  createTopic,
  updateTaskContent,
  applyTaskTransition,
  TaskExistsError,
} from "../api";
import styles from "./NewTaskDialog.module.css";

export type NewTaskDialogWarning = {
  stage: "content" | "plan_start";
  error: Error;
};

export interface NewTaskDialogResult {
  task: TaskEntry;
  plannerRequested: boolean;
  warning?: NewTaskDialogWarning;
}

interface NewTaskDialogProps {
  open: boolean;
  project: string;
  topics: TopicEntry[];
  onClose(): void;
  onCreated(result: NewTaskDialogResult): Promise<void> | void;
}

export default function NewTaskDialog({
  open,
  project,
  topics,
  onClose,
  onCreated,
}: NewTaskDialogProps) {
  const [description, setDescription] = useState("");
  const [filename, setFilename] = useState("");
  const [filenameTouched, setFilenameTouched] = useState(false);
  const [topicValue, setTopicValue] = useState("");
  const [topicIsNew, setTopicIsNew] = useState(false);
  const [kickOffPlanner, setKickOffPlanner] = useState(false);
  const [busy, setBusy] = useState(false);
  const [filenameError, setFilenameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Reset all state when the dialog opens so each open starts fresh.
  useEffect(() => {
    if (open) {
      setDescription("");
      setFilename("");
      setFilenameTouched(false);
      setTopicValue("");
      setTopicIsNew(false);
      setKickOffPlanner(false);
      setBusy(false);
      setFilenameError(null);
      setGeneralError(null);
    }
  }, [open]);

  // Focus the description textarea when the dialog opens.
  useEffect(() => {
    if (open) {
      const id = requestAnimationFrame(() => textareaRef.current?.focus());
      return () => cancelAnimationFrame(id);
    }
  }, [open]);

  // Close on Escape when not busy.
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) {
        e.preventDefault();
        onClose();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, busy, onClose]);

  // Derive filename from description when the user has not manually edited it.
  // Keep blank when description is empty so the submit button stays disabled.
  const derivedFilename = description.trim()
    ? deriveFilenameFromDescription(description)
    : "";
  const effectiveFilename = filenameTouched ? filename : derivedFilename;

  function handleDescriptionChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    setDescription(e.target.value);
  }

  function handleFilenameChange(e: React.ChangeEvent<HTMLInputElement>) {
    setFilename(sanitizeFilenameInput(e.target.value));
    setFilenameTouched(true);
    setFilenameError(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    // 1. Clear prior inline errors.
    setFilenameError(null);
    setGeneralError(null);

    setBusy(true);
    try {
      // 2. Create topic if the user declared a new one.
      if (topicIsNew && topicValue.trim() !== "") {
        try {
          await createTopic(project, topicValue.trim());
        } catch (err) {
          setGeneralError(err instanceof Error ? err.message : String(err));
          return;
        }
      }

      // 3. Create the task record.
      let task: TaskEntry;
      try {
        task = await createTask(project, {
          filename: effectiveFilename,
          description,
          topic: topicValue.trim() || undefined,
          branch: branchFromFilename(effectiveFilename),
          created_at: new Date().toISOString(),
        });
      } catch (err) {
        if (err instanceof TaskExistsError) {
          // 4. Duplicate filename — surface inline, keep dialog open.
          setFilenameError("filename already exists");
          return;
        }
        // 5. Other failure.
        setGeneralError(err instanceof Error ? err.message : String(err));
        return;
      }

      // 6. Write the initial stub content.
      try {
        await updateTaskContent(
          project,
          effectiveFilename,
          renderTaskStub(deriveTaskTitle(description), description, effectiveFilename),
        );
      } catch (err) {
        // 7. Content write failed — warn, but still report the created task.
        await onCreated({
          task,
          plannerRequested: false,
          warning: {
            stage: "content",
            error: err instanceof Error ? err : new Error(String(err)),
          },
        });
        onClose();
        return;
      }

      // 8. No planner requested — done.
      if (!kickOffPlanner) {
        await onCreated({ task, plannerRequested: false });
        onClose();
        return;
      }

      // 9. Kick off the planner.
      try {
        await applyTaskTransition(project, effectiveFilename, "plan_start");
      } catch (err) {
        // 10. Transition failed — warn, report that planner was requested.
        await onCreated({
          task,
          plannerRequested: true,
          warning: {
            stage: "plan_start",
            error: err instanceof Error ? err : new Error(String(err)),
          },
        });
        onClose();
        return;
      }

      // 11. Full success.
      await onCreated({ task, plannerRequested: true });
      onClose();
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;

  const canSubmit = effectiveFilename.trim() !== "" && !busy;

  return createPortal(
    <div
      className={styles.backdrop}
      onClick={busy ? undefined : onClose}
    >
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-task-title"
        aria-describedby="new-task-desc-hint"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="new-task-title" className={styles.title}>
          new task
        </h2>

        <form onSubmit={handleSubmit} className={styles.form}>
          <label className={styles.label} htmlFor="new-task-description">
            description
          </label>
          <textarea
            ref={textareaRef}
            id="new-task-description"
            className={styles.textarea}
            value={description}
            onChange={handleDescriptionChange}
            placeholder="what do you want to build?"
            rows={4}
            disabled={busy}
          />
          <span id="new-task-desc-hint" className={styles.srOnly}>
            enter a description for the new task
          </span>

          <label className={styles.label} htmlFor="new-task-topic">
            topic
          </label>
          <TopicCombobox
            topics={topics}
            value={topicValue}
            onChange={({ value, isNew }) => {
              setTopicValue(value);
              setTopicIsNew(isNew);
            }}
            disabled={busy}
            id="new-task-topic"
          />

          <label className={styles.label} htmlFor="new-task-filename">
            filename
          </label>
          <input
            id="new-task-filename"
            type="text"
            className={
              filenameError
                ? `${styles.input} ${styles.inputError}`
                : styles.input
            }
            value={effectiveFilename}
            onChange={handleFilenameChange}
            placeholder="task-slug"
            disabled={busy}
            aria-invalid={!!filenameError}
            aria-describedby={
              filenameError ? "new-task-filename-error" : undefined
            }
          />
          {filenameError && (
            <span
              id="new-task-filename-error"
              className={styles.fieldError}
              role="alert"
            >
              {filenameError}
            </span>
          )}

          <label className={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={kickOffPlanner}
              onChange={(e) => setKickOffPlanner(e.target.checked)}
              disabled={busy}
            />
            kick off planner now
          </label>

          {generalError && (
            <div className={styles.generalError} role="alert">
              {generalError}
            </div>
          )}

          <div className={styles.actions}>
            <button
              type="button"
              className={styles.cancelBtn}
              onClick={onClose}
              disabled={busy}
            >
              cancel
            </button>
            <button
              type="submit"
              className={styles.submitBtn}
              disabled={!canSubmit}
            >
              {busy ? "creating\u2026" : "create task"}
            </button>
          </div>
        </form>
      </div>
    </div>,
    document.body,
  );
}
