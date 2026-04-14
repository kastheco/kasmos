import { createPortal } from "react-dom";
import type { ToastItem } from "../hooks/useToast";
import styles from "./Toast.module.css";

interface ToastPresenterProps {
  toasts: ToastItem[];
  onDismiss: (id: number) => void;
}

export default function Toast({ toasts, onDismiss }: ToastPresenterProps) {
  if (toasts.length === 0) return null;

  return createPortal(
    <div className={styles.stack} aria-live="polite" aria-atomic="false">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`${styles.toast} ${t.kind === "error" ? styles.error : styles.success}`}
          role="status"
        >
          <span className={styles.message}>{t.message}</span>
          <button
            className={styles.dismiss}
            onClick={() => onDismiss(t.id)}
            aria-label="dismiss"
          >
            ×
          </button>
        </div>
      ))}
    </div>,
    document.body,
  );
}
