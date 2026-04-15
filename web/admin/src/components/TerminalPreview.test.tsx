// Pure tests for TerminalPreview using react-dom/server renderToStaticMarkup.
// No DOM, no browser, no test runner required — runs with `tsx` directly.

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import TerminalPreview, { buildPreviewHTML } from "./TerminalPreview.tsx";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

function assertContains(haystack: string, needle: string, msg: string): void {
  if (!haystack.includes(needle)) {
    throw new Error(`${msg}: expected to find ${JSON.stringify(needle)} in ${JSON.stringify(haystack)}`);
  }
}

function assertNotContains(haystack: string, needle: string, msg: string): void {
  if (haystack.includes(needle)) {
    throw new Error(`${msg}: expected NOT to find ${JSON.stringify(needle)} in ${JSON.stringify(haystack)}`);
  }
}

// ---- buildPreviewHTML -------------------------------------------------------

// ANSI color sequences are converted to <span> elements with inline styles.
const redText = "\x1b[31mhello\x1b[0m";
const redHtml = buildPreviewHTML(redText);
assertContains(redHtml, "<span", "ANSI color produces span element");
assertContains(redHtml, "hello", "ANSI color preserves text content");
// The raw escape sequence must not appear in output.
assertNotContains(redHtml, "\x1b[31m", "raw ANSI escape is removed");

// Raw HTML tags in the input must be escaped so scripts cannot be injected.
const xssInput = '<script>alert("xss")</script>';
const xssHtml = buildPreviewHTML(xssInput);
assertNotContains(xssHtml, "<script>", "raw <script> tag is escaped");
assertContains(xssHtml, "&lt;script&gt;", "script tag is HTML-escaped");

// Line trimming: only the last maxLines lines are retained.
const lines = Array.from({ length: 60 }, (_, i) => `line${i}`).join("\n");
const trimmedHtml = buildPreviewHTML(lines, 10);
assertNotContains(trimmedHtml, "line0", "leading lines are trimmed");
assertContains(trimmedHtml, "line59", "last line is retained after trim");

// When content is shorter than maxLines, all lines pass through.
const shortContent = "a\nb\nc";
const shortHtml = buildPreviewHTML(shortContent, 10);
assertContains(shortHtml, "a", "short content: first line present");
assertContains(shortHtml, "c", "short content: last line present");

// ---- TerminalPreview component (renderToStaticMarkup) ----------------------

// Empty content renders empty state label.
const emptyMarkup = renderToStaticMarkup(
  <TerminalPreview content="" />,
);
assertContains(emptyMarkup, "no output yet", "empty content shows default empty label");
assertContains(emptyMarkup, "<pre", "empty content still renders pre wrapper");

// Custom emptyLabel is used when provided.
const customEmptyMarkup = renderToStaticMarkup(
  <TerminalPreview content="   " emptyLabel="waiting for output" />,
);
assertContains(customEmptyMarkup, "waiting for output", "custom emptyLabel is rendered");

// ANSI color spans appear in component output.
// Use {expression} not "string" so ESC char is interpreted by JS, not JSX.
const colorMarkup = renderToStaticMarkup(
  <TerminalPreview content={"\x1b[32mgreen\x1b[0m"} />,
);
assertContains(colorMarkup, "<span", "component renders ANSI color spans");
assertContains(colorMarkup, "green", "component preserves text content");

// XSS: raw HTML in content is escaped before insertion.
const xssMarkup = renderToStaticMarkup(
  <TerminalPreview content={'<img src=x onerror="alert(1)">'} />,
);
assertNotContains(xssMarkup, "<img", "component escapes raw HTML tags");
assertContains(xssMarkup, "&lt;img", "component HTML-encodes < in content");

// Line trimming: maxLines prop limits output to last N lines.
const manyLines = Array.from({ length: 50 }, (_, i) => `L${i}`).join("\n");
const trimMarkup = renderToStaticMarkup(
  <TerminalPreview content={manyLines} maxLines={5} />,
);
assertNotContains(trimMarkup, "L0", "component trims leading lines");
assertContains(trimMarkup, "L49", "component retains last line after trim");

console.log("TerminalPreview.test.tsx ok");
