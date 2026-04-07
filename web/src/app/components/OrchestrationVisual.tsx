import styles from "./OrchestrationVisual.module.css";

interface OrchestrationVisualProps {
  className?: string;
}

export default function OrchestrationVisual({ className }: OrchestrationVisualProps) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 160 160"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={`${styles.visual} ${className ?? ""}`}
    >
      {/* Connecting lines from center to peripheral nodes */}
      <line x1="80" y1="80" x2="30" y2="30" className={`${styles.line} ${styles.foam}`} />
      <line x1="80" y1="80" x2="130" y2="30" className={`${styles.line} ${styles.iris}`} />
      <line x1="80" y1="80" x2="20" y2="100" className={`${styles.line} ${styles.gold}`} />
      <line x1="80" y1="80" x2="140" y2="100" className={`${styles.line} ${styles.foam} ${styles.delay1}`} />
      <line x1="80" y1="80" x2="60" y2="145" className={`${styles.line} ${styles.iris} ${styles.delay2}`} />
      <line x1="80" y1="80" x2="110" y2="145" className={`${styles.line} ${styles.gold} ${styles.delay3}`} />

      {/* Peripheral agent nodes */}
      <circle cx="30" cy="30" r="7" className={`${styles.node} ${styles.foam}`} />
      <circle cx="130" cy="30" r="5" className={`${styles.node} ${styles.iris} ${styles.delay1}`} />
      <circle cx="20" cy="100" r="6" className={`${styles.node} ${styles.gold} ${styles.delay2}`} />
      <circle cx="140" cy="100" r="5" className={`${styles.node} ${styles.foam} ${styles.delay3}`} />
      <circle cx="60" cy="145" r="6" className={`${styles.node} ${styles.iris} ${styles.delay4}`} />
      <circle cx="110" cy="145" r="5" className={`${styles.node} ${styles.gold} ${styles.delay5}`} />

      {/* Central orchestrator node */}
      <circle cx="80" cy="80" r="12" className={styles.center} />
    </svg>
  );
}
