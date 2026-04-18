import styles from "./rows.module.css";

/**
 * Renders the "response" label + rule that separates tool activity from the
 * assistant's final prose output in a turn. `row.kind === "response"` always
 * maps here — it is never degraded to a generic text row.
 */
export function ResponseDivider() {
  return (
    <div className={styles.responseDivider} data-kind="response">
      <span className={styles.responseDividerLabel}>response</span>
      <hr className={styles.responseDividerRule} />
    </div>
  );
}
