import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import styles from "./PromptDialog.module.css";

export interface PromptDialogProps {
  open: boolean;
  title: string;
  label: string;
  initialValue: string;
  placeholder?: string;
  multiline?: boolean;
  submitLabel?: string;
  allowEmpty?: boolean;
  onSubmit: (value: string) => void;
  onCancel: () => void;
}

export default function PromptDialog({
  open,
  title,
  label,
  initialValue,
  placeholder,
  multiline = false,
  submitLabel = "save",
  allowEmpty = false,
  onSubmit,
  onCancel,
}: PromptDialogProps) {
  const [value, setValue] = useState(initialValue);
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null);

  // Sync value when dialog opens with a new initialValue
  useEffect(() => {
    if (open) {
      setValue(initialValue);
    }
  }, [open, initialValue]);

  useEffect(() => {
    if (open) {
      // Let the render settle before focusing
      const id = requestAnimationFrame(() => inputRef.current?.focus());
      return () => cancelAnimationFrame(id);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onCancel();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onCancel]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (allowEmpty || value.trim()) {
      onSubmit(value);
    }
  };

  if (!open) return null;

  return createPortal(
    <div className={styles.backdrop} onClick={onCancel}>
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="prompt-title"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="prompt-title" className={styles.title}>
          {title}
        </h2>
        <form onSubmit={handleSubmit} className={styles.form}>
          <label className={styles.label} htmlFor="prompt-input">
            {label}
          </label>
          {multiline ? (
            <textarea
              ref={(node) => {
                inputRef.current = node;
              }}
              id="prompt-input"
              className={styles.textarea}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={placeholder}
              rows={5}
            />
          ) : (
            <input
              ref={(node) => {
                inputRef.current = node;
              }}
              id="prompt-input"
              type="text"
              className={styles.input}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={placeholder}
            />
          )}
          <div className={styles.actions}>
            <button type="button" className={styles.cancelBtn} onClick={onCancel}>
              cancel
            </button>
            <button
              type="submit"
              className={styles.submitBtn}
              disabled={!allowEmpty && !value.trim()}
            >
              {submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>,
    document.body,
  );
}
