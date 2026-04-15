/**
 * Client-side ports of TUI task-creation helpers.
 * These are pure functions with no React or browser coupling.
 *
 * Go counterparts:
 *   deriveTaskTitle   → app/task_title.go:heuristicPlanTitle
 *   slugifyTaskName   → app/app_state.go:slugifyPlanName / buildPlanFilename
 *   branchFromFilename → session/git/task_lifecycle.go:TaskBranchFromFile
 *   renderTaskStub    → app/app_state.go:renderPlanStub
 */

function splitWords(s: string): string[] {
  return s.split(/\s+/).filter((w) => w !== "");
}

/**
 * Derives a short title from a plan description.
 * Mirrors app/task_title.go:heuristicPlanTitle (lines 19-72).
 */
export function deriveTaskTitle(description: string): string {
  let text = description.trim();
  if (!text) return "new plan";

  // Take first line only
  const nl = text.indexOf("\n");
  if (nl >= 0) {
    text = text.slice(0, nl).trim();
  }
  if (!text) return "new plan";

  // Strip common filler prefixes (case-insensitive)
  const fillers = [
    "i want to ",
    "i'd like to ",
    "we need to ",
    "we should ",
    "please ",
    "let's ",
    "let us ",
    "can you ",
    "could you ",
  ];
  const lower = text.toLowerCase();
  for (const f of fillers) {
    if (lower.startsWith(f)) {
      text = text.slice(f.length);
      break;
    }
  }
  text = text.trim();
  if (!text) return "new plan";

  const words = splitWords(text);
  if (words.length <= 6) return words.join(" ");

  // Look for a natural break within first 8 words
  const limit = Math.min(8, words.length);
  const first8 = words.slice(0, limit).join(" ");
  for (const sep of [", ", "; ", ": ", ". ", " - "]) {
    const idx = first8.indexOf(sep);
    if (idx > 0) {
      const candidate = first8.slice(0, idx).trim();
      if (splitWords(candidate).length >= 3) return candidate;
    }
  }

  // No natural break — truncate to 6 words
  return words.slice(0, 6).join(" ");
}

/**
 * Converts a name to a URL-safe slug, using `fallback` when the result is empty.
 * Mirrors app/app_state.go:slugifyPlanName + buildPlanFilename (lines 2707-2721).
 */
export function slugifyTaskName(name: string, fallback = "plan"): string {
  const slug = name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || fallback;
}

/**
 * Like slugifyTaskName but returns "" instead of "plan" on empty input.
 * Used while the user is still editing the filename field so the field can be
 * empty without a placeholder appearing.
 */
export function sanitizeFilenameInput(input: string): string {
  return input
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/**
 * Derives a filesystem-safe filename from a description by combining
 * deriveTaskTitle and slugifyTaskName.
 */
export function deriveFilenameFromDescription(description: string): string {
  return slugifyTaskName(deriveTaskTitle(description));
}

/**
 * Derives the git branch name for an already-slugged filename.
 * Mirrors session/git/task_lifecycle.go:TaskBranchFromFile (lines 14-20).
 */
export function branchFromFilename(filename: string): string {
  return "plan/" + slugifyTaskName(filename);
}

/**
 * Returns the initial markdown content for a new task file.
 * Mirrors app/app_state.go:renderPlanStub (lines 2724-2726).
 */
export function renderTaskStub(
  title: string,
  description: string,
  filename: string,
): string {
  return `# ${title}\n\n## Context\n\n${description}\n\n## Notes\n\n- Created by kas lifecycle flow\n- Plan file: ${filename}\n`;
}
