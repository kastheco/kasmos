// agentCardModel parses an InstanceEntry into a display-ready agent card
// model. Used by InstancesPage so the UI can surface a deslugified base name
// plus role/wave/task pills instead of raw slugs like
// "instance-interactivity-W1-T1".
//
// Parsing prefers structured fields on the InstanceEntry (agent_type,
// wave_number, task_number, task_file) over regex-parsing the title. Title
// regex is kept as a back-compat fallback for entries produced by older
// daemons that didn't populate those fields.

import type { InstanceEntry, InstanceStatus } from "../types";

export interface AgentPill {
  label: string;
  tone?: "default" | "wave" | "task" | "cycle" | "role";
}

export interface AgentCardModel {
  /** Original title — used as React key and as a selection identifier. */
  title: string;
  /** Deslugified, human-friendly plan / solo name. */
  displayName: string;
  /** Status pill — drives the top-level badge and grouping. */
  status: InstanceStatus;
  /** Short, space-separated pills (role, wave, task, cycle). */
  pills: AgentPill[];
  /** Optional branch line shown as a dim meta row. */
  branch?: string;
  /** Optional last-updated timestamp (ISO string) for the meta row. */
  updatedAt?: string;
  /** Program string, e.g. "claude" / "opencode" — shown in meta. */
  program?: string;
}

/** Status groups — display order left-to-right / top-to-bottom. */
export const STATUS_GROUPS: InstanceStatus[] = [
  "running",
  "loading",
  "ready",
  "paused",
];

/** Section headers shown above each status group. Uses the app's lowercase
 *  aesthetic (kasmos CLAUDE.md rule). */
export const STATUS_GROUP_LABELS: Record<InstanceStatus, string> = {
  running: "running",
  loading: "loading",
  ready: "ready",
  paused: "paused",
};

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
export function toAgentCardModel(inst: InstanceEntry): AgentCardModel {
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
    pills,
    branch: inst.branch || undefined,
    updatedAt: inst.updated_at,
    program: inst.program,
  };
}

/** Group agent cards by status in STATUS_GROUPS order. Empty groups are
 *  dropped so the UI doesn't show "ready (0)" when there's nothing to
 *  render. */
export function groupAgentsByStatus(
  agents: AgentCardModel[],
): { status: InstanceStatus; label: string; cards: AgentCardModel[] }[] {
  const buckets: Record<InstanceStatus, AgentCardModel[]> = {
    running: [],
    loading: [],
    ready: [],
    paused: [],
  };
  for (const a of agents) {
    if (buckets[a.status]) {
      buckets[a.status].push(a);
    } else {
      // Unknown status — drop into running so it stays visible.
      buckets.running.push(a);
    }
  }
  return STATUS_GROUPS.map((status) => ({
    status,
    label: STATUS_GROUP_LABELS[status],
    cards: buckets[status],
  })).filter((g) => g.cards.length > 0);
}
