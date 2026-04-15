// Unit coverage for the agent card model parser used by the admin agents
// page. Runs as a pure script via tsx — no DOM, no React renderer.

import {
  deslugify,
  groupAgentsByStatus,
  toAgentCardModel,
} from "./agentCardModel.ts";
import type { InstanceEntry } from "../types.ts";

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

// --- groupAgentsByStatus: ordering + empty group filtering -----------------

{
  const cards = [
    {
      title: "a",
      displayName: "a",
      status: "ready" as const,
      pills: [],
    },
    {
      title: "b",
      displayName: "b",
      status: "running" as const,
      pills: [],
    },
    {
      title: "c",
      displayName: "c",
      status: "running" as const,
      pills: [],
    },
    {
      title: "d",
      displayName: "d",
      status: "loading" as const,
      pills: [],
    },
  ];
  const groups = groupAgentsByStatus(cards);
  // Order must be running → loading → ready → paused, and paused must be
  // dropped because it has no cards.
  assertDeepEqual(
    groups.map((g) => [g.status, g.cards.length]),
    [
      ["running", 2],
      ["loading", 1],
      ["ready", 1],
    ],
    "status grouping order and empty filter",
  );
  assertEqual(groups[0].cards[0].title, "b", "running preserves insertion order");
  assertEqual(groups[0].cards[1].title, "c", "running preserves insertion order");
}

console.log("agentCardModel.test.ts ok");
