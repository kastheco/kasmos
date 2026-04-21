import { useState } from "react";
import type { PresentationTurn } from "../../types";
import type { AgentPreviewFilters } from "./FilterToolbar";
import { PermissionCard } from "./PermissionCard";
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

/** Rows that are hidden by collapse (but not by filters). */
const COLLAPSE_HIDDEN_KINDS = new Set(["thinking", "tool", "tool_diff", "result", "tool_preview", "system"]);

/** Rows hidden by the hideTools filter. */
const TOOLS_FILTER_KINDS = new Set(["tool", "tool_diff", "result", "tool_preview"]);

/**
 * Produce a plain-text copy summary for a turn.
 * Permission rows are omitted; response rows become a "---" separator.
 */
function buildCopyText(turn: PresentationTurn, elapsedLabel: string | null): string {
  const lines: string[] = [`turn ${turn.number}`];
  if (elapsedLabel) lines.push(`elapsed: ${elapsedLabel}`);
  for (const row of turn.rows) {
    if (row.kind === "response") {
      lines.push("---");
    } else if (row.kind === "permission") {
      // omit permission from copy
    } else if (row.kind === "tool") {
      lines.push(`[${row.tool_name || "tool"}] ${row.text}`);
    } else if (row.kind === "tool_diff") {
      const payload = row.tool_diff;
      if (payload) {
        if (payload.path) lines.push(`diff: ${payload.path}`);
        for (const line of payload.lines ?? []) {
          const prefix = line.kind === "added" ? "+" : line.kind === "removed" ? "-" : " ";
          const text = line.kind === "removed"
            ? (line.old_text ?? "")
            : (line.new_text ?? line.old_text ?? "");
          lines.push(`${prefix}${text}`);
        }
        if (payload.truncated) lines.push(`… ${payload.hidden_line_count ?? 0} lines hidden`);
      }
    } else if (row.kind === "tool_preview") {
      const payload = row.tool_preview;
      if (payload) {
        for (const line of payload.lines ?? []) lines.push(line);
        if (payload.truncated) lines.push(`… ${payload.hidden_line_count ?? 0} lines hidden`);
      }
    } else {
      lines.push(row.text);
    }
  }
  return lines.join("\n");
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
  /** Project name, forwarded to PermissionCard for the permission POST. */
  project: string;
  /** Instance title, forwarded to PermissionCard for the permission POST. */
  title: string;
  /** Global filter state from FilterToolbar. */
  filters: AgentPreviewFilters;
  /**
   * Number of permission rows in turns that precede this one in the snapshot.
   * The first unresolved permission card globally (offset 0) is interactive.
   */
  permissionOffset: number;
}

/**
 * Renders a single presentation turn with:
 * - turn header (#N, elapsed, tool count, running pill)
 * - collapse toggle (session-local, does not persist)
 * - copy button (writes plain-text summary to clipboard; shows fallback on error)
 * - anchor link (#turn-N, updates location.hash)
 * - global filter application (hideTools / hideThinking / hideSystem)
 * - PermissionCard for permission rows (only first unresolved is interactive)
 *
 * Collapse hides thinking/tool/result/system rows.
 * Global filters never hide permission rows.
 */
export function TurnBlock({
  turn,
  capturedAt,
  snapshotReceivedAt,
  project,
  title,
  filters,
  permissionOffset,
}: TurnBlockProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [copyFallback, setCopyFallback] = useState<string | null>(null);

  const isRunning = turn.completed_at === null && !turn.interrupted;

  // ---------------------------------------------------------------------------
  // Elapsed time
  // ---------------------------------------------------------------------------

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

  // ---------------------------------------------------------------------------
  // Copy handler
  // ---------------------------------------------------------------------------

  async function handleCopy() {
    const text = buildCopyText(turn, elapsedLabel);
    try {
      await navigator.clipboard.writeText(text);
      setCopyFallback(null);
    } catch {
      // Clipboard API unavailable (e.g. non-secure context) — show inline fallback.
      setCopyFallback(text);
    }
  }

  // ---------------------------------------------------------------------------
  // Anchor handler
  // ---------------------------------------------------------------------------

  function handleAnchor(e: React.MouseEvent<HTMLAnchorElement>) {
    e.preventDefault();
    const hash = `turn-${turn.number}`;
    location.hash = hash;
    document.getElementById(hash)?.scrollIntoView({ behavior: "smooth" });
  }

  // ---------------------------------------------------------------------------
  // Row visibility
  // ---------------------------------------------------------------------------

  /** Whether a row should be rendered at all. Permission rows are always visible. */
  function isRowVisible(kind: string): boolean {
    if (kind === "permission") return true; // never hidden
    if (collapsed && COLLAPSE_HIDDEN_KINDS.has(kind)) return false;
    if (!collapsed) {
      if (filters.hideThinking && kind === "thinking") return false;
      if (filters.hideTools && TOOLS_FILTER_KINDS.has(kind)) return false;
      if (filters.hideSystem && kind === "system") return false;
    }
    return true;
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  // Track permission row count within this turn to compute interactive state.
  let localPermCount = 0;

  const renderedRows = turn.rows.map((row, i) => {
    if (!isRowVisible(row.kind)) return null;

    if (row.kind === "permission") {
      const globalIndex = permissionOffset + localPermCount;
      localPermCount++;
      return (
        <PermissionCard
          key={i}
          text={row.text}
          project={project}
          title={title}
          interactive={globalIndex === 0}
        />
      );
    }

    return renderRow(row, i);
  });

  return (
    <div id={`turn-${turn.number}`} className={styles.turnBlock}>
      <div className={styles.turnHeader}>
        <span className={styles.turnNumber}>#{turn.number}</span>
        {elapsedLabel && (
          <span className={styles.turnMeta}>{elapsedLabel}</span>
        )}
        {toolLabel && (
          <span className={styles.turnMeta}>{toolLabel}</span>
        )}
        {isRunning && (
          <span className={styles.runningPill}>
            {turn.activity?.label
              ? `• running · ${turn.activity.label}`
              : "• running"}
          </span>
        )}

        {/* header controls */}
        <span className={styles.turnHeaderControls}>
          <a
            className={styles.turnAnchor}
            href={`#turn-${turn.number}`}
            onClick={handleAnchor}
            aria-label={`anchor to turn ${turn.number}`}
          >
            #{turn.number}
          </a>
          <button
            className={styles.turnHeaderBtn}
            onClick={handleCopy}
            aria-label="copy turn"
          >
            copy
          </button>
          <button
            className={styles.turnHeaderBtn}
            onClick={() => {
              setCollapsed((c) => !c);
              setCopyFallback(null);
            }}
            aria-label={collapsed ? "expand turn" : "collapse turn"}
          >
            {collapsed ? "expand" : "collapse"}
          </button>
        </span>
      </div>

      {copyFallback !== null && (
        <div className={styles.copyFallback}>
          <span className={styles.copyFallbackLabel}>copy manually:</span>
          <textarea
            className={styles.copyFallbackArea}
            readOnly
            value={copyFallback}
            rows={Math.min(copyFallback.split("\n").length + 1, 8)}
            onClick={(e) => (e.target as HTMLTextAreaElement).select()}
          />
          <button
            className={styles.turnHeaderBtn}
            onClick={() => setCopyFallback(null)}
          >
            dismiss
          </button>
        </div>
      )}

      {turn.rows.length > 0 && (
        <div className={styles.turnRows}>{renderedRows}</div>
      )}
    </div>
  );
}
