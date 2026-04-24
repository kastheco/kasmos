// agentCardModel parses an InstanceEntry into a display-ready agent card
// model. Used by InstancesPage so the UI can surface a deslugified base name
// plus role/wave/task pills instead of raw slugs like
// "instance-interactivity-W1-T1".
//
// Parsing prefers structured fields on the InstanceEntry (agent_type,
// wave_number, task_number, task_file) over regex-parsing the title. Title
// regex is kept as a back-compat fallback for entries produced by older
// daemons that didn't populate those fields.

import type { InstanceEntry, InstanceStatus, Status } from "../types";

export interface AgentPill {
  label: string;
  tone?: "default" | "wave" | "task" | "cycle" | "role";
}

export interface AgentCardModel {
  /** Original title — used as React key and as a selection identifier. */
  title: string;
  /** Deslugified, human-friendly plan / solo name. */
  displayName: string;
  /** Live instance substatus — still drives the per-card badge. Intentionally
   *  NOT used for grouping anymore: a running↔ready flicker would reshuffle
   *  the list on every poll. */
  status: InstanceStatus;
  presentation: "active" | "retired" | "idle";
  /** Plan / task filename this agent is working on, if any. Solo agents
   *  leave this undefined. Used for grouping + sorting. */
  taskFile?: string;
  /** Wave number (1-based), if this is a wave task agent. */
  waveNumber?: number;
  /** Task number (1-based), if this is a wave task agent. */
  taskNumber?: number;
  /** Short, space-separated pills (role, wave, task, cycle). */
  pills: AgentPill[];
  /** Optional branch line shown as a dim meta row. */
  branch?: string;
  /** Optional last-updated timestamp (ISO string) for the meta row. */
  updatedAt?: string;
  /** Program string, e.g. "claude" / "opencode" — shown in meta. */
  program?: string;
}

/** Task-status groups — display order top-to-bottom. Matches the plan
 *  lifecycle so running work appears above ready/done work. */
export const TASK_STATUS_GROUPS: Status[] = [
  "reviewing",
  "verifying",
  "implementing",
  "planning",
  "ready",
  "done",
  "cancelled",
];

/** Sentinel key for agents with no attached task (solo / ad-hoc agents). */
export const SOLO_GROUP_KEY = "solo";

/** Section headers shown above each group. Uses the app's lowercase
 *  aesthetic (kasmos CLAUDE.md rule). */
export const TASK_STATUS_GROUP_LABELS: Record<Status, string> = {
  reviewing: "reviewing",
  verifying: "verifying",
  implementing: "implementing",
  planning: "planning",
  ready: "ready",
  done: "done",
  cancelled: "cancelled",
};

export const SOLO_GROUP_LABEL = "agents";

/** Deslugify a kebab/underscore/dot-separated identifier into a lowercase
 *  space-separated label. Empty input returns "". */
export function deslugify(raw: string | undefined): string {
  if (!raw) return "";
  return raw
    .trim()
    .replace(/[-_.]+/g, " ")
    .replace(/\s+/g, " ")
    .toLowerCase();
}

const TITLE_SUFFIX_PATTERNS: {
  suffix: RegExp;
  pill: (match: RegExpMatchArray) => AgentPill[];
}[] = [
  // <plan>-W<wave>-T<task>
  {
    suffix: /-W(\d+)-T(\d+)$/i,
    pill: (m) => [
      { label: "coder", tone: "role" },
      { label: `wave ${m[1]}`, tone: "wave" },
      { label: `task ${m[2]}`, tone: "task" },
    ],
  },
  // <plan>-review-<cycle>
  {
    suffix: /-review-(\d+)$/i,
    pill: (m) => [
      { label: "reviewer", tone: "role" },
      { label: `cycle ${m[1]}`, tone: "cycle" },
    ],
  },
  // <plan>-fix-<cycle>
  {
    suffix: /-fix-(\d+)$/i,
    pill: (m) => [
      { label: "fixer", tone: "role" },
      { label: `cycle ${m[1]}`, tone: "cycle" },
    ],
  },
  // <plan>-verify-<cycle> (master agent)
  {
    suffix: /-verify-(\d+)$/i,
    pill: (m) => [
      { label: "master", tone: "role" },
      { label: `cycle ${m[1]}`, tone: "cycle" },
    ],
  },
  // <plan>-plan
  {
    suffix: /-plan$/i,
    pill: () => [{ label: "planner", tone: "role" }],
  },
  // <plan>-architect (canonical) or <plan>-elaborator (legacy)
  {
    suffix: /-(architect|elaborator)$/i,
    pill: () => [{ label: "architect", tone: "role" }],
  },
  // <plan>-coder (single-agent implement)
  {
    suffix: /-coder$/i,
    pill: () => [{ label: "coder", tone: "role" }],
  },
];

/** Map an agent_type string to a canonical user-facing role label. */
function roleFromAgentType(agentType: string | undefined): string | undefined {
  if (!agentType) return undefined;
  const norm = agentType.trim().toLowerCase();
  switch (norm) {
    case "planner":
      return "planner";
    case "elaborator":
    case "architect":
      return "architect";
    case "coder":
      return "coder";
    case "reviewer":
      return "reviewer";
    case "fixer":
      return "fixer";
    case "master":
      return "master";
    default:
      return norm;
  }
}

/** Parse a single instance into a display model. */
export function deriveAgentPresentation(
  inst: InstanceEntry,
  taskStatus?: Status,
): "active" | "retired" | "idle" {
  if ((inst.status as string) === "exited" || taskStatus === "done" || taskStatus === "cancelled") {
    return "retired";
  }
  if (inst.status === "paused" && taskStatus !== "reviewing" && taskStatus !== "implementing" && taskStatus !== "verifying") {
    return "idle";
  }
  return "active";
}

/** Parse a single instance into a display model. */
export function toAgentCardModel(inst: InstanceEntry, taskStatus?: Status): AgentCardModel {
  const pills: AgentPill[] = [];

  // Prefer structured fields when the daemon populated them.
  const role = roleFromAgentType(inst.agent_type);
  const hasWaveTask =
    typeof inst.wave_number === "number" && inst.wave_number > 0 &&
    typeof inst.task_number === "number" && inst.task_number > 0;

  if (hasWaveTask) {
    pills.push({ label: role ?? "coder", tone: "role" });
    pills.push({ label: `wave ${inst.wave_number}`, tone: "wave" });
    pills.push({ label: `task ${inst.task_number}`, tone: "task" });
  } else if (role) {
    pills.push({ label: role, tone: "role" });
  }

  // Derive the display name: prefer task_file (the plan), otherwise strip
  // the known role suffix from the title and use that. Solo/standalone
  // agents with no recognised suffix fall back to the raw title.
  let baseSlug = inst.task_file ?? "";
  if (!baseSlug) {
    baseSlug = inst.title;
    for (const p of TITLE_SUFFIX_PATTERNS) {
      const m = baseSlug.match(p.suffix);
      if (m) {
        baseSlug = baseSlug.slice(0, m.index ?? baseSlug.length);
        // When we fell back to regex parsing AND we didn't already add
        // pills from structured fields, use the regex-derived pills.
        if (pills.length === 0) {
          pills.push(...p.pill(m));
        }
        break;
      }
    }
  }

  const displayName = deslugify(baseSlug) || inst.title;

  return {
    title: inst.title,
    displayName,
    status: inst.status,
    presentation: deriveAgentPresentation(inst, taskStatus),
    taskFile: inst.task_file || undefined,
    waveNumber:
      typeof inst.wave_number === "number" && inst.wave_number > 0
        ? inst.wave_number
        : undefined,
    taskNumber:
      typeof inst.task_number === "number" && inst.task_number > 0
        ? inst.task_number
        : undefined,
    pills,
    branch: inst.branch || undefined,
    updatedAt: inst.updated_at,
    program: inst.program,
  };
}

export interface AgentGroup {
  /** Machine key — either a plan Status or SOLO_GROUP_KEY. */
  key: Status | typeof SOLO_GROUP_KEY;
  /** Human label rendered in the section header. */
  label: string;
  cards: AgentCardModel[];
}

/** Stable compare used within a task-status bucket. Agents for the same plan
 *  stay together (alphabetical by task_file), then wave number ascending,
 *  then task number ascending. Ties fall back to title so the order is
 *  fully deterministic and does not depend on fetch order. */
function compareAgentsWithinGroup(a: AgentCardModel, b: AgentCardModel): number {
  const fa = a.taskFile ?? "";
  const fb = b.taskFile ?? "";
  if (fa !== fb) return fa.localeCompare(fb);
  const wa = a.waveNumber ?? Number.MAX_SAFE_INTEGER;
  const wb = b.waveNumber ?? Number.MAX_SAFE_INTEGER;
  if (wa !== wb) return wa - wb;
  const ta = a.taskNumber ?? Number.MAX_SAFE_INTEGER;
  const tb = b.taskNumber ?? Number.MAX_SAFE_INTEGER;
  if (ta !== tb) return ta - tb;
  return a.title.localeCompare(b.title);
}

/** Group agent cards by the plan lifecycle status of their attached task —
 *  NOT by the live instance substatus (running / ready / …) which flips on
 *  every poll tick and was causing visible reshuffling. Agents without a
 *  task_file (solo / ad-hoc) are collected into a trailing "agents" group.
 *  Within each group, cards are sorted stably by (task_file, wave, task #).
 *
 *  `taskStatusByFile` maps task filename → plan status. Instances whose task
 *  is not present in the map (task was deleted, or tasks haven't loaded yet)
 *  fall back to "ready" so they remain visible and group deterministically.
 */
export function groupAgentsByTaskStatus(
  agents: AgentCardModel[],
  taskStatusByFile: ReadonlyMap<string, Status>,
): AgentGroup[] {
  const taskBuckets = new Map<Status, AgentCardModel[]>();
  for (const s of TASK_STATUS_GROUPS) taskBuckets.set(s, []);
  const soloBucket: AgentCardModel[] = [];

  for (const a of agents) {
    if (!a.taskFile) {
      soloBucket.push(a);
      continue;
    }
    const taskStatus = taskStatusByFile.get(a.taskFile) ?? "ready";
    const bucket = taskBuckets.get(taskStatus) ?? taskBuckets.get("ready")!;
    bucket.push(a);
  }

  const groups: AgentGroup[] = [];
  for (const status of TASK_STATUS_GROUPS) {
    const cards = taskBuckets.get(status)!;
    if (cards.length === 0) continue;
    cards.sort(compareAgentsWithinGroup);
    groups.push({
      key: status,
      label: TASK_STATUS_GROUP_LABELS[status],
      cards,
    });
  }
  if (soloBucket.length > 0) {
    soloBucket.sort(compareAgentsWithinGroup);
    groups.push({ key: SOLO_GROUP_KEY, label: SOLO_GROUP_LABEL, cards: soloBucket });
  }
  return groups;
}
