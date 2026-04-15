import React from "react";
import { AnsiUp } from "ansi_up";
import styles from "./TerminalPreview.module.css";

/**
 * buildPreviewHTML converts an ANSI-escaped terminal string to safe HTML.
 * Trims to the last `maxLines` lines before conversion so large panes
 * do not explode the DOM. When maxLines <= 0, all lines are retained
 * (unbounded — used for the "full" scrollback depth preset).
 * Exported for pure unit testing without a DOM.
 */
export function buildPreviewHTML(content: string, maxLines = 40): string {
  const ansi = new AnsiUp();
  ansi.escape_html = true;
  ansi.use_classes = false;

  const lines = content.split("\n");
  const trimmed =
    maxLines > 0 && lines.length > maxLines
      ? lines.slice(lines.length - maxLines)
      : lines;
  return ansi.ansi_to_html(trimmed.join("\n"));
}

interface TerminalPreviewProps {
  content: string;
  maxLines?: number;
  emptyLabel?: string;
  onScroll?: React.UIEventHandler<HTMLPreElement>;
}

const TerminalPreview = React.forwardRef<HTMLPreElement, TerminalPreviewProps>(
  function TerminalPreview(
    { content, maxLines = 40, emptyLabel = "no output yet", onScroll },
    ref,
  ) {
    if (!content.trim()) {
      return (
        <pre ref={ref} className={styles.terminal} onScroll={onScroll}>
          <span className={styles.empty}>{emptyLabel}</span>
        </pre>
      );
    }

    const html = buildPreviewHTML(content, maxLines);

    return (
      <pre
        ref={ref}
        className={styles.terminal}
        onScroll={onScroll}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{ __html: html }}
      />
    );
  },
);

export default TerminalPreview;
