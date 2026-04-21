import type { ReactNode } from "react";
import type { PresentationRow, ToolDiffLineKind } from "../../types";
import { ResponseDivider } from "./ResponseDivider";
import { ProseMarkdown } from "./ProseMarkdown";
import { formatToolLabel, limitToolPreview, splitToolText } from "./toolFormatting";
import styles from "./rows.module.css";

// ---------------------------------------------------------------------------
// Internal row renderers
// ---------------------------------------------------------------------------

interface RowProps {
  row: PresentationRow;
}

function rowKindClass(row: PresentationRow): string {
  switch (row.kind) {
    case "user":       return styles.kindUser;
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
    case "user":       return "you";
    case "thinking":   return "thinking";
    case "tool":       return formatToolLabel(row.tool_name);
    case "result":     return "result";
    case "system":     return "system";
    case "permission": return "permission";
    case "status":     return "status";
    default:           return "";
  }
}

function diffLineClass(kind: ToolDiffLineKind): string {
  switch (kind) {
    case "added":   return styles.diffLineAdded;
    case "removed": return styles.diffLineRemoved;
    default:        return styles.diffLineContext;
  }
}

function TextRow({ row }: RowProps) {
  const kindClass = rowKindClass(row);
  const prefix = rowPrefix(row);
  const toolText = row.kind === "tool" ? splitToolText(row.text, row.tool_name) : null;
  const detail = toolText ? toolText.detail : row.text;
  return (
    <div
      className={`${styles.row} ${kindClass}`}
      data-kind={row.kind}
      data-error={row.is_error ? "true" : undefined}
    >
      {prefix && <span className={styles.rowKind}>{prefix}</span>}
      <span className={`${styles.rowText} ${row.kind === "tool" ? styles.toolDetail : ""}`}>{detail}</span>
    </div>
  );
}

function ProseRow({ row }: RowProps) {
  return (
    <div className={`${styles.row} ${styles.kindProse}`} data-kind="prose">
      <div className={styles.rowText}>
        <ProseMarkdown text={row.text} />
      </div>
    </div>
  );
}

function DiffRow({ row }: RowProps) {
  const payload = row.tool_diff;
  if (!payload) return null;
  return (
    <div className={`${styles.row} ${styles.kindDiff}`} data-kind="tool_diff">
      <span className={styles.rowKind}>diff</span>
      <span className={styles.rowText}>
        {payload.path && (
          <div className={styles.diffPath}>{payload.path}</div>
        )}
        {(payload.lines ?? []).map((line, i) => (
          <div key={i} className={`${styles.diffLine} ${diffLineClass(line.kind)}`}>
            <span className={styles.diffGutter}>
              {line.kind === "removed"
                ? (line.old_number ?? "")
                : (line.new_number ?? line.old_number ?? "")}
            </span>
            <span className={styles.diffLineContent}>
              {line.kind === "removed"
                ? (line.old_text ?? "")
                : (line.new_text ?? line.old_text ?? "")}
            </span>
          </div>
        ))}
        {payload.truncated && (
          <div className={styles.diffTruncated}>
            {`… ${payload.hidden_line_count ?? 0} lines hidden`}
          </div>
        )}
      </span>
    </div>
  );
}

function PreviewRow({ row }: RowProps) {
  const payload = row.tool_preview;
  if (!payload) return null;
  const preview = limitToolPreview(payload);
  const text = preview.lines.join("\n");
  return (
    <div className={`${styles.row} ${styles.kindPreview}`} data-kind="tool_preview">
      <span className={styles.rowKind}>preview</span>
      <span className={styles.rowText}>
        <span className={styles.previewLines}>{text}</span>
        {preview.truncated && (
          <div className={styles.previewTruncated}>
            {`… ${preview.hiddenLineCount} lines hidden`}
          </div>
        )}
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public switch: row.kind → renderer
//
// NOTE: `permission` rows are handled by TurnBlock (which wraps them in
// PermissionCard with project/title/interactive context).  If renderRow is
// called with a permission row it falls through to TextRow as a safe fallback.
// ---------------------------------------------------------------------------

/**
 * Maps a single `PresentationRow` to its React node.
 * `response` rows always become `ResponseDivider` — never generic text.
 * `prose` rows render as react-markdown (remark-gfm, no raw HTML passthrough).
 * `tool_diff` rows render as a structured inline diff with gutter and colors.
 * `tool_preview` rows render as a whitespace-preserving text preview.
 * All other rows are monospace text rows with a dimmed kind prefix.
 */
export function renderRow(row: PresentationRow, index: number): ReactNode {
  switch (row.kind) {
    case "response":
      return <ResponseDivider key={index} />;
    case "prose":
      return <ProseRow key={index} row={row} />;
    case "tool_diff":
      return <DiffRow key={index} row={row} />;
    case "tool_preview":
      return <PreviewRow key={index} row={row} />;
    default:
      return <TextRow key={index} row={row} />;
  }
}
