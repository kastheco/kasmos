import type { ReactNode } from "react";
import type { PresentationRow } from "../../types";
import { ResponseDivider } from "./ResponseDivider";
import styles from "./rows.module.css";

// ---------------------------------------------------------------------------
// Internal row renderers
// ---------------------------------------------------------------------------

interface RowProps {
  row: PresentationRow;
}

function rowKindClass(row: PresentationRow): string {
  switch (row.kind) {
    case "thinking":   return styles.kindThinking;
    case "tool":       return styles.kindTool;
    case "result":     return row.is_error ? styles.kindResultError : styles.kindResult;
    case "system":     return styles.kindSystem;
    case "permission": return styles.kindPermission;
    case "status":     return styles.kindStatus;
    default:           return "";
  }
}

function rowPrefix(row: PresentationRow): string {
  switch (row.kind) {
    case "thinking":   return "thinking";
    case "tool":       return row.tool_name || "tool";
    case "result":     return "result";
    case "system":     return "system";
    case "permission": return "permission";
    case "status":     return "status";
    default:           return "";
  }
}

function TextRow({ row }: RowProps) {
  const kindClass = rowKindClass(row);
  const prefix = rowPrefix(row);
  return (
    <div
      className={`${styles.row} ${kindClass}`}
      data-kind={row.kind}
      data-error={row.is_error ? "true" : undefined}
    >
      {prefix && <span className={styles.rowKind}>{prefix}</span>}
      <span className={styles.rowText}>{row.text}</span>
    </div>
  );
}

function ProseRow({ row }: RowProps) {
  return (
    <div className={`${styles.row} ${styles.kindProse}`} data-kind="prose">
      <span className={styles.rowText}>{row.text}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public switch: row.kind → renderer
// ---------------------------------------------------------------------------

/**
 * Maps a single `PresentationRow` to its React node.
 * `response` rows always become `ResponseDivider` — never generic text.
 * `prose` rows use `white-space: pre-wrap`.
 * All other rows are monospace text rows with a dimmed kind prefix.
 */
export function renderRow(row: PresentationRow, index: number): ReactNode {
  switch (row.kind) {
    case "response":
      return <ResponseDivider key={index} />;
    case "prose":
      return <ProseRow key={index} row={row} />;
    default:
      return <TextRow key={index} row={row} />;
  }
}
