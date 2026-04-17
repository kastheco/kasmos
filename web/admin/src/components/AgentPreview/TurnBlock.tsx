import type { PresentationTurn } from "../../types";
import { renderRow } from "./rows";
import styles from "./AgentPreview.module.css";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatElapsedMs(ms: number): string {
  if (ms < 1000) return "<1s";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  return rs > 0 ? `${m}m ${rs}s` : `${m}m`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface TurnBlockProps {
  turn: PresentationTurn;
  /** Server-side snapshot timestamp, used for elapsed of running turns. */
  capturedAt: Date;
  /** Browser timestamp when the snapshot was received (ms since epoch). */
  snapshotReceivedAt: number;
}

/**
 * Renders a single presentation turn: turn header + typed rows.
 * Running turns (completed_at === null && !interrupted) show the • running pill.
 * Interrupted turns rely on the status row from the server — no extra badge.
 */
export function TurnBlock({ turn, capturedAt, snapshotReceivedAt }: TurnBlockProps) {
  const isRunning = turn.completed_at === null && !turn.interrupted;

  // Elapsed time for display.
  let elapsedLabel: string | null = null;
  if (turn.started_at) {
    const baseMs = isRunning
      ? capturedAt.getTime() - turn.started_at.getTime() + (Date.now() - snapshotReceivedAt)
      : turn.completed_at
        ? turn.completed_at.getTime() - turn.started_at.getTime()
        : null;
    if (baseMs !== null && baseMs >= 0) {
      elapsedLabel = formatElapsedMs(baseMs);
    }
  }

  const toolLabel =
    turn.tool_count > 0
      ? `${turn.tool_count} tool${turn.tool_count !== 1 ? "s" : ""}`
      : null;

  return (
    <div className={styles.turnBlock}>
      <div className={styles.turnHeader}>
        <span className={styles.turnNumber}>#{turn.number}</span>
        {elapsedLabel && (
          <span className={styles.turnMeta}>{elapsedLabel}</span>
        )}
        {toolLabel && (
          <span className={styles.turnMeta}>{toolLabel}</span>
        )}
        {isRunning && (
          <span className={styles.runningPill}>• running</span>
        )}
      </div>

      {turn.rows.length > 0 && (
        <div className={styles.turnRows}>
          {turn.rows.map((row, i) => renderRow(row, i))}
        </div>
      )}
    </div>
  );
}
