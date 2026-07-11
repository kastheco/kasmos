import type { LifecycleCounts } from "./types";
import styles from "./widget.module.css";

export default function LifecycleRail({ lifecycle }: { lifecycle: LifecycleCounts }) {
  const entries = ["planning", "ready", "implementing", "reviewing", "verifying"] as const;
  return <div className={styles.lifecycle} role="list" aria-label="lifecycle counts">
    {entries.map((status) => <span role="listitem" className={styles.chip} key={status}><b>{lifecycle[status]}</b> {status}</span>)}
  </div>;
}
