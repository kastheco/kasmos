import React, { useRef } from "react";
import { AnsiUp } from "ansi_up";
import styles from "./TerminalPreview.module.css";

/**
 * buildPreviewHTML converts an ANSI-escaped terminal string to safe HTML.
 * Trims to the last `maxLines` lines before conversion so large panes
 * do not explode the DOM. Exported for pure unit testing without a DOM.
 */
export function buildPreviewHTML(content: string, maxLines = 40): string {
  const ansi = new AnsiUp();
  ansi.escape_html = true;
  ansi.use_classes = false;

  const lines = content.split("\n");
  const trimmed =
    lines.length > maxLines ? lines.slice(lines.length - maxLines) : lines;
  return ansi.ansi_to_html(trimmed.join("\n"));
}

interface TerminalPreviewProps {
  content: string;
  maxLines?: number;
  emptyLabel?: string;
}

export default function TerminalPreview({
  content,
  maxLines = 40,
  emptyLabel = "no output yet",
}: TerminalPreviewProps): React.ReactElement {
  const ansiRef = useRef<AnsiUp | null>(null);
  if (!ansiRef.current) {
    ansiRef.current = new AnsiUp();
    ansiRef.current.escape_html = true;
    ansiRef.current.use_classes = false;
  }

  if (!content.trim()) {
    return <pre className={styles.terminal}><span className={styles.empty}>{emptyLabel}</span></pre>;
  }

  const lines = content.split("\n");
  const trimmed =
    lines.length > maxLines ? lines.slice(lines.length - maxLines) : lines;
  const html = ansiRef.current.ansi_to_html(trimmed.join("\n"));

  return (
    <pre
      className={styles.terminal}
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
