// Unit tests for taskCreation helpers — run with `tsx src/lib/taskCreation.test.ts`.
// No test runner required: helpers are pure functions.

import {
  deriveTaskTitle,
  slugifyTaskName,
  sanitizeFilenameInput,
  deriveFilenameFromDescription,
  branchFromFilename,
  renderTaskStub,
} from "./taskCreation.ts";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

// ---- deriveTaskTitle -------------------------------------------------------

// Empty / whitespace input returns sentinel
assertEqual(deriveTaskTitle(""), "new plan", "empty → new plan");
assertEqual(deriveTaskTitle("   "), "new plan", "whitespace-only → new plan");

// Filler prefixes are stripped
assertEqual(
  deriveTaskTitle("please add a login button"),
  "add a login button",
  "strips 'please '",
);
assertEqual(
  deriveTaskTitle("we need to refactor the auth module"),
  "refactor the auth module",
  "strips 'we need to '",
);
assertEqual(
  deriveTaskTitle("can you build a settings page"),
  "build a settings page",
  "strips 'can you '",
);
assertEqual(
  deriveTaskTitle("i want to improve performance"),
  "improve performance",
  "strips 'i want to '",
);
assertEqual(
  deriveTaskTitle("could you write unit tests"),
  "write unit tests",
  "strips 'could you '",
);

// Filler stripping is case-insensitive
assertEqual(
  deriveTaskTitle("Please add dark mode"),
  "add dark mode",
  "case-insensitive filler strip",
);

// Short description (≤6 words) is returned as-is after filler strip
assertEqual(
  deriveTaskTitle("fix the broken login flow"),
  "fix the broken login flow",
  "5-word description unchanged",
);

// Long description truncates to 6 words
assertEqual(
  deriveTaskTitle("add a comprehensive error handling system for the entire application"),
  "add a comprehensive error handling system",
  "long single-line truncated to 6 words",
);

// Natural break at "," — candidate must have ≥3 words before the separator
assertEqual(
  deriveTaskTitle(
    "add oauth support, update session management, remove legacy tokens",
  ),
  "add oauth support",
  "truncates at comma when candidate has ≥3 words",
);

// Natural break candidate with fewer than 3 words is skipped — falls back to 6-word truncation
assertEqual(
  deriveTaskTitle("auth, add better error handling with rich context info"),
  "auth, add better error handling with",
  "skips short comma candidate (<3 words), falls back to 6-word truncation",
);

// Multiline description: only the first line is used
assertEqual(
  deriveTaskTitle("Fix login bug\n\nThis is the detailed description\nSpanning multiple lines"),
  "Fix login bug",
  "multiline: only first line used",
);
assertEqual(
  deriveTaskTitle("please fix the login bug\ndetails here"),
  "fix the login bug",
  "multiline with filler prefix stripped",
);

// A description with only whitespace on first line — trim() collapses leading whitespace
// so "   \nSecond line" becomes "Second line" (treated as single-line input)
assertEqual(
  deriveTaskTitle("   \nSecond line only"),
  "Second line only",
  "leading whitespace-newline trimmed; remaining treated as single line",
);

// ---- slugifyTaskName -------------------------------------------------------

assertEqual(slugifyTaskName("Auth Refactor"), "auth-refactor", "basic slug");
assertEqual(slugifyTaskName("  leading spaces  "), "leading-spaces", "trims spaces");
assertEqual(slugifyTaskName("hello---world"), "hello-world", "collapses dashes");
assertEqual(slugifyTaskName("café au lait"), "caf-au-lait", "removes non-ascii");
assertEqual(slugifyTaskName("---"), "plan", "dash-only falls back to 'plan'");
assertEqual(slugifyTaskName(""), "plan", "empty falls back to 'plan'");
assertEqual(slugifyTaskName("!!!"), "plan", "symbol-only falls back to 'plan'");
assertEqual(
  slugifyTaskName("", "custom"),
  "custom",
  "custom fallback respected",
);

// ---- sanitizeFilenameInput -------------------------------------------------

assertEqual(
  sanitizeFilenameInput("Auth Refactor"),
  "auth-refactor",
  "sanitize basic",
);
assertEqual(sanitizeFilenameInput(""), "", "empty returns empty (no fallback)");
assertEqual(sanitizeFilenameInput("   "), "", "whitespace-only returns empty");
assertEqual(sanitizeFilenameInput("!!!"), "", "symbol-only returns empty");
assertEqual(
  sanitizeFilenameInput("---"),
  "",
  "dash-only returns empty",
);

// ---- deriveFilenameFromDescription ----------------------------------------

assertEqual(
  deriveFilenameFromDescription("please add a dark mode toggle"),
  "add-a-dark-mode-toggle",
  "combines deriveTaskTitle + slugifyTaskName",
);
assertEqual(
  deriveFilenameFromDescription(""),
  "new-plan",
  "empty description → 'new-plan' slug",
);

// ---- branchFromFilename ----------------------------------------------------

assertEqual(
  branchFromFilename("auth-refactor"),
  "plan/auth-refactor",
  "branch from slug",
);
assertEqual(
  branchFromFilename("My Feature"),
  "plan/my-feature",
  "branch normalises input",
);
assertEqual(
  branchFromFilename(""),
  "plan/plan",
  "empty filename → plan/plan",
);

// ---- renderTaskStub --------------------------------------------------------

const stub = renderTaskStub("Auth Refactor", "Improve auth flow", "auth-refactor");
if (!stub.startsWith("# Auth Refactor\n")) {
  throw new Error(`renderTaskStub: expected h1 title, got: ${stub}`);
}
if (!stub.includes("## Context\n\nImprove auth flow\n")) {
  throw new Error(`renderTaskStub: expected Context section, got: ${stub}`);
}
if (!stub.includes("## Notes\n\n- Created by kas lifecycle flow\n- Plan file: auth-refactor\n")) {
  throw new Error(`renderTaskStub: expected Notes section, got: ${stub}`);
}

// Byte-for-byte check against Go's renderPlanStub format
const expected =
  "# Auth Refactor\n\n## Context\n\nImprove auth flow\n\n## Notes\n\n- Created by kas lifecycle flow\n- Plan file: auth-refactor\n";
assertEqual(stub, expected, "renderTaskStub byte-for-byte match");

console.log("taskCreation.test.ts ok");
