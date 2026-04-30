// Unit coverage for the agent card model parser used by the admin agents
// page. Runs as a pure script via tsx — no DOM, no React renderer.

import {
  deslugify,
  deriveAgentPresentation,
  groupAgentsByTaskStatus,
  toAgentCardModel,
} from "./agentCardModel.ts";
import type { InstanceEntry, Status } from "../types.ts";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

function assertDeepEqual<T>(actual: T, expected: T, msg: string): void {
  const a = JSON.stringify(actual);
  const b = JSON.stringify(expected);
  if (a !== b) {
    throw new Error(`${msg}:\n  expected ${b}\n  got      ${a}`);
  }
}

// --- deslugify -------------------------------------------------------------

assertEqual(
  deslugify("instance-interactivity"),
  "instance interactivity",
  "deslugify kebab",
);
assertEqual(
  deslugify("instance_interactivity"),
  "instance interactivity",
  "deslugify snake",
);
assertEqual(
  deslugify("multi-word_with.dots"),
  "multi word with dots",
  "deslugify mixed separators",
);
assertEqual(deslugify(""), "", "deslugify empty");
assertEqual(deslugify(undefined), "", "deslugify undefined");

// --- presentation derivation -------------------------------------------------

{
  const base: InstanceEntry = {
    title: "agent",
    status: "running",
    branch: "",
    program: "claude",
    task_file: "feature",
  };
  const cases: Array<{
    name: string;
    inst: InstanceEntry;
    taskStatus?: Status;
    want: "active" | "retired" | "idle";
  }> = [
    { name: "running implementing", inst: base, taskStatus: "implementing", want: "active" },
    { name: "running done task", inst: base, taskStatus: "done", want: "active" },
    { name: "ready done task", inst: { ...base, status: "ready" }, taskStatus: "done", want: "active" },
    { name: "running cancelled task", inst: base, taskStatus: "cancelled", want: "active" },
    { name: "paused ready", inst: { ...base, status: "paused" }, taskStatus: "ready", want: "idle" },
    { name: "paused done task", inst: { ...base, status: "paused" }, taskStatus: "done", want: "idle" },
    { name: "paused reviewing", inst: { ...base, status: "paused" }, taskStatus: "reviewing", want: "active" },
    { name: "exited instance", inst: { ...base, status: "exited" as InstanceEntry["status"] }, taskStatus: "implementing", want: "retired" },
    { name: "exited done task", inst: { ...base, status: "exited" as InstanceEntry["status"] }, taskStatus: "done", want: "retired" },
  ];
  for (const tc of cases) {
    assertEqual(deriveAgentPresentation(tc.inst, tc.taskStatus), tc.want, tc.name);
    assertEqual(toAgentCardModel(tc.inst, tc.taskStatus).presentation, tc.want, `${tc.name} card`);
  }
}

// --- wave-task parsing from structured fields ------------------------------

{
  const inst: InstanceEntry = {
    title: "instance-interactivity-W1-T2",
    status: "running",
    branch: "plan/instance-interactivity",
    program: "claude",
    task_file: "instance-interactivity",
    agent_type: "coder",
    wave_number: 1,
    task_number: 2,
  };
  const card = toAgentCardModel(inst);
  assertEqual(
    card.displayName,
    "instance interactivity",
    "wave task uses task_file for display",
  );
  assertDeepEqual(
    card.pills.map((p) => [p.label, p.tone]),
    [
      ["coder", "role"],
      ["wave 1", "wave"],
      ["task 2", "task"],
    ],
    "wave task pills include role/wave/task",
  );
}

// --- planner from agent_type -----------------------------------------------

{
  const inst: InstanceEntry = {
    title: "create-plan-view-plan",
    status: "running",
    branch: "",
    program: "claude",
    task_file: "create-plan-view",
    agent_type: "planner",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.displayName, "create plan view", "planner display name");
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["planner"],
    "planner pill",
  );
}

// --- reviewer with agent_type but no cycle info (no cycle pill from fields) -

{
  const inst: InstanceEntry = {
    title: "feature-review-3",
    status: "running",
    branch: "plan/feature",
    program: "claude",
    task_file: "feature",
    agent_type: "reviewer",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.displayName, "feature", "reviewer display name");
  // structured path only has agent_type so cycle is unknown without title parse
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["reviewer"],
    "reviewer pill (structured path)",
  );
}

// --- title regex fallback for legacy entries with no structured fields ----

{
  const inst: InstanceEntry = {
    title: "legacy-plan-W2-T5",
    status: "loading",
    branch: "plan/legacy-plan",
    program: "claude",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.displayName, "legacy plan", "regex fallback display");
  assertDeepEqual(
    card.pills.map((p) => [p.label, p.tone]),
    [
      ["coder", "role"],
      ["wave 2", "wave"],
      ["task 5", "task"],
    ],
    "regex fallback pills",
  );
}

{
  const inst: InstanceEntry = {
    title: "legacy-plan-review-2",
    status: "running",
    branch: "",
    program: "claude",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.displayName, "legacy plan", "reviewer regex display");
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["reviewer", "cycle 2"],
    "reviewer regex pills",
  );
}

{
  const inst: InstanceEntry = {
    title: "legacy-plan-verify-1",
    status: "running",
    branch: "",
    program: "claude",
  };
  const card = toAgentCardModel(inst);
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["master", "cycle 1"],
    "master regex pills",
  );
}

{
  const inst: InstanceEntry = {
    title: "legacy-plan-elaborator",
    status: "running",
    branch: "",
    program: "claude",
  };
  const card = toAgentCardModel(inst);
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["architect"],
    "elaborator alias normalised to architect",
  );
}

// --- standalone/solo with no structured fields and no recognised suffix ---

{
  const inst: InstanceEntry = {
    title: "kasmos-agent-1",
    status: "running",
    branch: "",
    program: "claude --permission-mode bypassPermissions",
    agent_type: "fixer",
  };
  const card = toAgentCardModel(inst);
  // agent_type=fixer but no task_file → fall through to title deslugify
  assertEqual(card.displayName, "kasmos agent 1", "standalone display");
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["fixer"],
    "standalone fixer pill",
  );
}

// --- unknown agent_type surfaces as a role pill passthrough ---------------

{
  const inst: InstanceEntry = {
    title: "experiment-something",
    status: "running",
    branch: "",
    program: "claude",
    agent_type: "explorer",
  };
  const card = toAgentCardModel(inst);
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["explorer"],
    "unknown agent_type passthrough",
  );
}

// --- groupAgentsByTaskStatus: groups by plan lifecycle, not live status ----

{
  // Two plans, one actively implementing, one already reviewing — plus a
  // solo agent with no task_file. Each plan has two wave/task agents so we
  // can also verify the wave/task tiebreaker ordering inside a group.
  const cards = [
    // Agents for plan "alpha" which is currently reviewing.
    {
      title: "alpha-W2-T1",
      displayName: "alpha",
      status: "ready" as const,
      taskFile: "alpha",
      waveNumber: 2,
      taskNumber: 1,
      presentation: "active" as const,
      pills: [],
    },
    {
      title: "alpha-W1-T1",
      displayName: "alpha",
      status: "running" as const,
      taskFile: "alpha",
      waveNumber: 1,
      taskNumber: 1,
      presentation: "active" as const,
      pills: [],
    },
    // Agents for plan "beta" which is currently implementing.
    {
      title: "beta-W1-T2",
      displayName: "beta",
      status: "running" as const,
      taskFile: "beta",
      waveNumber: 1,
      taskNumber: 2,
      presentation: "active" as const,
      pills: [],
    },
    {
      title: "beta-W1-T1",
      displayName: "beta",
      status: "paused" as const,
      taskFile: "beta",
      waveNumber: 1,
      taskNumber: 1,
      presentation: "active" as const,
      pills: [],
    },
    // Solo agent (no task_file) — falls into the trailing "agents" group.
    {
      title: "adhoc",
      displayName: "adhoc",
      status: "running" as const,
      presentation: "active" as const,
      pills: [],
    },
  ];
  const taskStatus: ReadonlyMap<string, Status> = new Map<string, Status>([
    ["alpha", "reviewing"],
    ["beta", "implementing"],
  ]);
  const groups = groupAgentsByTaskStatus(cards, taskStatus);

  // Reviewing comes before implementing (per TASK_STATUS_GROUPS order); solo
  // trails at the bottom. Empty lifecycle buckets are dropped.
  assertDeepEqual(
    groups.map((g) => [g.key, g.cards.length]),
    [
      ["reviewing", 2],
      ["implementing", 2],
      ["solo", 1],
    ],
    "groups by plan lifecycle status, not live instance status",
  );

  // Within a bucket: sorted by (task_file, wave, task) — NOT by insertion
  // order and NOT by instance substatus.
  assertEqual(groups[0].cards[0].title, "alpha-W1-T1", "reviewing[0] = wave 1 task 1");
  assertEqual(groups[0].cards[1].title, "alpha-W2-T1", "reviewing[1] = wave 2 task 1");
  assertEqual(groups[1].cards[0].title, "beta-W1-T1", "implementing[0] = wave 1 task 1");
  assertEqual(groups[1].cards[1].title, "beta-W1-T2", "implementing[1] = wave 1 task 2");
  assertEqual(groups[2].cards[0].title, "adhoc", "solo group contains ad-hoc agent");
}

// --- groupAgentsByTaskStatus: stability across live status flips ----------

{
  // Same card in two different "polls" — substatus flips running ↔ ready.
  // The bucket (task status) must stay the same across both passes.
  const makeCard = (substatus: "running" | "ready") => [
    {
      title: "plan-W1-T1",
      displayName: "plan",
      status: substatus,
      taskFile: "plan",
      waveNumber: 1,
      taskNumber: 1,
      presentation: "active" as const,
      pills: [],
    },
  ];
  const taskStatus: ReadonlyMap<string, Status> = new Map<string, Status>([
    ["plan", "implementing"],
  ]);

  const before = groupAgentsByTaskStatus(makeCard("running"), taskStatus);
  const after = groupAgentsByTaskStatus(makeCard("ready"), taskStatus);

  assertDeepEqual(
    before.map((g) => g.key),
    after.map((g) => g.key),
    "instance substatus change must not reshuffle group layout",
  );
  assertEqual(before[0].key, "implementing", "bucket is plan lifecycle, not live status");
}

// --- groupAgentsByTaskStatus: unknown task_file falls back to ready --------

{
  const cards = [
    {
      title: "orphan",
      displayName: "orphan",
      status: "running" as const,
      taskFile: "missing-plan",
      presentation: "active" as const,
      pills: [],
    },
  ];
  const groups = groupAgentsByTaskStatus(cards, new Map());
  assertEqual(groups.length, 1, "orphan still renders");
  assertEqual(groups[0].key, "ready", "unknown task_file falls back to ready bucket");
}

// --- resource_profile mapping --------------------------------------------------

{
  // Non-normal profile is forwarded to the card model.
  const inst: InstanceEntry = {
    title: "plan-coder",
    status: "running",
    branch: "plan/feature",
    program: "claude",
    task_file: "feature",
    agent_type: "coder",
    resource_profile: "interactive",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.resourceProfile, "interactive", "resource_profile forwarded to card");
}

{
  // Absent resource_profile maps to undefined (not an empty string).
  const inst: InstanceEntry = {
    title: "plan-coder",
    status: "running",
    branch: "",
    program: "claude",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.resourceProfile, undefined, "absent resource_profile maps to undefined");
}

// --- solo pill: ad-hoc agent with no role ----------------------------------

{
  const inst: InstanceEntry = {
    title: "my-solo-agent",
    status: "running",
    branch: "feat/solo",
    program: "claude",
    solo_agent: true,
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.pills[0].label, "solo", "ad-hoc solo: first pill is solo");
  assertEqual(card.pills[0].tone, "default", "ad-hoc solo: solo pill tone is default");
  assertEqual(card.pills.length, 1, "ad-hoc solo with no role: only solo pill");
}

// --- solo pill: solo agent with agent_type ---------------------------------

{
  const inst: InstanceEntry = {
    title: "solo-fixer-agent",
    status: "running",
    branch: "feat/fix",
    program: "claude",
    solo_agent: true,
    agent_type: "fixer",
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.pills[0].label, "solo", "solo+agent_type: first pill is solo");
  assertEqual(card.pills[1].label, "fixer", "solo+agent_type: second pill is role");
  assertEqual(card.pills.length, 2, "solo+agent_type: exactly two pills");
}

// --- solo pill: solo wave/task row ----------------------------------------

{
  const inst: InstanceEntry = {
    title: "solo-plan-W1-T2",
    status: "running",
    branch: "plan/solo-plan",
    program: "claude",
    task_file: "solo-plan",
    agent_type: "coder",
    wave_number: 1,
    task_number: 2,
    solo_agent: true,
  };
  const card = toAgentCardModel(inst);
  assertDeepEqual(
    card.pills.map((p) => [p.label, p.tone]),
    [
      ["solo", "default"],
      ["coder", "role"],
      ["wave 1", "wave"],
      ["task 2", "task"],
    ],
    "solo wave/task row: solo pill first, then role/wave/task",
  );
}

// --- solo pill: legacy title-derived solo row -----------------------------

{
  // A legacy entry with a recognised suffix but also solo_agent=true.
  // The solo pill must still appear first; regex-derived pills follow.
  const inst: InstanceEntry = {
    title: "adhoc-plan-review-1",
    status: "running",
    branch: "",
    program: "claude",
    solo_agent: true,
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.pills[0].label, "solo", "legacy solo: first pill is solo");
  assertEqual(card.pills[1].label, "reviewer", "legacy solo: second pill is reviewer");
  assertEqual(card.pills[2].label, "cycle 1", "legacy solo: third pill is cycle");
  assertEqual(card.pills.length, 3, "legacy solo: three pills total");
}

// --- solo pill: plain coder negative case (no solo_agent) -----------------

{
  const inst: InstanceEntry = {
    title: "plan-W1-T1",
    status: "running",
    branch: "plan/plan",
    program: "claude",
    task_file: "plan",
    agent_type: "coder",
    wave_number: 1,
    task_number: 1,
  };
  const card = toAgentCardModel(inst);
  assertEqual(card.pills[0].label, "coder", "no solo_agent: first pill is coder not solo");
  assertDeepEqual(
    card.pills.map((p) => p.label),
    ["coder", "wave 1", "task 1"],
    "no solo_agent: no solo pill",
  );
}

// --- groupAgentsByTaskStatus: solo task-attached rows stay in task buckets -

{
  // A solo agent that also has a task_file must NOT be moved to the
  // trailing "agents" group — only rows without task_file belong there.
  const cards = [
    {
      title: "solo-attached",
      displayName: "solo attached",
      status: "running" as const,
      taskFile: "my-plan",
      presentation: "active" as const,
      pills: [{ label: "solo", tone: "default" as const }],
    },
    {
      title: "pure-solo",
      displayName: "pure solo",
      status: "running" as const,
      presentation: "active" as const,
      pills: [{ label: "solo", tone: "default" as const }],
    },
  ];
  const taskStatus: ReadonlyMap<string, Status> = new Map<string, Status>([
    ["my-plan", "implementing"],
  ]);
  const groups = groupAgentsByTaskStatus(cards, taskStatus);
  assertEqual(groups.length, 2, "solo attached + pure solo: two groups");
  assertEqual(groups[0].key, "implementing", "solo attached stays in implementing bucket");
  assertEqual(groups[0].cards[0].title, "solo-attached", "solo attached card in implementing");
  assertEqual(groups[1].key, "solo", "pure solo goes to solo group");
  assertEqual(groups[1].cards[0].title, "pure-solo", "pure solo card in solo group");
}

console.log("agentCardModel.test.ts ok");
