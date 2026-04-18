import { useState } from "react";
import { sendInstancePermission } from "../../api";
import type { PermissionDecision } from "../../types";
import styles from "./PermissionCard.module.css";

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface PermissionCardProps {
  text: string;
  project: string;
  title: string;
  /** True only for the first unresolved permission row in the snapshot. */
  interactive: boolean;
}

/**
 * Inline permission prompt with allow / deny / always actions.
 *
 * State machine:
 *  - idle: buttons enabled
 *  - pending: all buttons disabled while POST is in-flight
 *  - dismissed: card hidden (optimistic; server reconciles on next poll)
 *  - error: buttons re-enabled, inline error shown
 *
 * A poll that removes the row before the POST resolves is treated as success —
 * the local dismissed state persists.
 *
 * When `interactive` is false the card renders read-only (no buttons).
 */
export function PermissionCard({
  text,
  project,
  title,
  interactive,
}: PermissionCardProps) {
  const [pending, setPending] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (dismissed) return null;

  if (!interactive) {
    return (
      <div className={styles.card} data-interactive="false" data-kind="permission">
        <span className={styles.text}>{text}</span>
        <span className={styles.readOnly}>waiting for earlier decision</span>
      </div>
    );
  }

  async function choose(decision: PermissionDecision) {
    setPending(true);
    setError(null);
    try {
      await sendInstancePermission(project, title, decision);
      setDismissed(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "error sending permission";
      setError(msg.toLowerCase());
      setPending(false);
    }
  }

  return (
    <div className={styles.card} data-interactive="true" data-kind="permission">
      <span className={styles.text}>{text}</span>
      <div className={styles.actions}>
        <button
          className={styles.btn}
          onClick={() => choose("allow_once")}
          disabled={pending}
        >
          allow
        </button>
        <button
          className={`${styles.btn} ${styles.btnAlways}`}
          onClick={() => choose("allow_always")}
          disabled={pending}
        >
          always
        </button>
        <button
          className={`${styles.btn} ${styles.btnDeny}`}
          onClick={() => choose("reject")}
          disabled={pending}
        >
          deny
        </button>
      </div>
      {error && <span className={styles.error}>{error}</span>}
    </div>
  );
}
