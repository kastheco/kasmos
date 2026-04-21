import type { ToolPreviewPayload } from "../../types";

export const MAX_TOOL_PREVIEW_LINES = 10;

function isCommandExecution(toolName: string): boolean {
  return toolName.trim().toLowerCase() === "commandexecution";
}

export function formatToolLabel(toolName: string): string {
  const trimmed = toolName.trim();
  if (isCommandExecution(trimmed)) return "ran";
  return trimmed || "tool";
}

export function splitToolText(text: string, toolName: string): { label: string; detail: string } {
  const label = formatToolLabel(toolName);
  const trimmedText = text.trim();

  if (isCommandExecution(toolName)) {
    if (trimmedText.startsWith("• ")) {
      return { label, detail: trimmedText.slice(2) };
    }
    return { label, detail: trimmedText };
  }

  const rawName = toolName.trim();
  if (trimmedText !== "" && rawName !== "") {
    const prefix = `• ${rawName}`;
    if (trimmedText === prefix) {
      return { label, detail: "" };
    }
    if (trimmedText.startsWith(`${prefix} `)) {
      return { label, detail: trimmedText.slice(prefix.length + 1) };
    }
  }

  return { label, detail: text };
}

export function formatToolCopyText(text: string, toolName: string): string {
  const { label, detail } = splitToolText(text, toolName);
  return detail ? `[${label}] ${detail}` : `[${label}]`;
}

export function limitToolPreview(payload?: ToolPreviewPayload | null): {
  lines: string[];
  truncated: boolean;
  hiddenLineCount: number;
} {
  const lines = payload?.lines ?? [];
  const clientHidden = Math.max(0, lines.length - MAX_TOOL_PREVIEW_LINES);

  return {
    lines: lines.slice(0, MAX_TOOL_PREVIEW_LINES),
    truncated: Boolean(payload?.truncated) || clientHidden > 0,
    hiddenLineCount: (payload?.hidden_line_count ?? 0) + clientHidden,
  };
}
