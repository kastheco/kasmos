// Pure DOM-free tests for InstanceActionsMenu helper logic.
// Covers action ordering and enabled/disabled state calculation.
// Runs via `tsx` (same pattern as api.test.ts) — no test framework required.

import type { InstanceEntry, InstanceAction } from "../types.ts";
import {
  getMenuItems,
  reduceMenuKey,
  shouldFireAction,
  type InstanceMenuItem,
} from "./InstanceActionsMenu.tsx";
import { routeInstanceAction } from "../pages/InstancesPage.tsx";

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
    throw new Error(`${msg}: expected ${b}, got ${a}`);
  }
}

// ---- action ordering --------------------------------------------------------

// getMenuItems always returns items in the canonical order: pause, resume,
// restart, kill — regardless of valid_actions content.
const allActions: InstanceEntry = {
  title: "test",
  status: "running",
  branch: "main",
  program: "claude",
  valid_actions: ["kill", "pause", "restart", "resume"],
};
const allItems = getMenuItems(allActions);
assertEqual(allItems.length, 4, "returns exactly 4 items");
assertEqual(allItems[0].action, "pause" as InstanceAction, "first item is pause");
assertEqual(allItems[1].action, "resume" as InstanceAction, "second item is resume");
assertEqual(allItems[2].action, "restart" as InstanceAction, "third item is restart");
assertEqual(allItems[3].action, "kill" as InstanceAction, "fourth item is kill");

// ---- enabled / disabled calculation -----------------------------------------

// All items are enabled when all actions are in valid_actions.
for (const item of allItems) {
  assertEqual(item.enabled, true, `${item.action} should be enabled`);
}

// Kill is disabled when not in valid_actions.
const noKill: InstanceEntry = {
  title: "test",
  status: "running",
  branch: "main",
  program: "claude",
  valid_actions: ["pause", "resume", "restart"],
};
const noKillItems = getMenuItems(noKill);
assertEqual(noKillItems.find((i) => i.action === "kill")!.enabled, false, "kill disabled when not in valid_actions");
assertEqual(noKillItems.find((i) => i.action === "pause")!.enabled, true, "pause still enabled");

// All items disabled when valid_actions is empty.
const noneAllowed: InstanceEntry = {
  title: "test",
  status: "paused",
  branch: "main",
  program: "claude",
  valid_actions: [],
};
for (const item of getMenuItems(noneAllowed)) {
  assertEqual(item.enabled, false, `${item.action} disabled when valid_actions is empty`);
}

// All items disabled when valid_actions is absent.
const noField: InstanceEntry = {
  title: "test",
  status: "ready",
  branch: "main",
  program: "claude",
};
for (const item of getMenuItems(noField)) {
  assertEqual(item.enabled, false, `${item.action} disabled when valid_actions is absent`);
}

// Only resume enabled for a paused instance.
const pausedInstance: InstanceEntry = {
  title: "planner-1",
  status: "paused",
  branch: "feat/x",
  program: "claude",
  valid_actions: ["resume", "kill"],
};
const pausedItems = getMenuItems(pausedInstance);
assertEqual(pausedItems.find((i) => i.action === "resume")!.enabled, true, "resume enabled for paused");
assertEqual(pausedItems.find((i) => i.action === "pause")!.enabled, false, "pause disabled for paused");
assertEqual(pausedItems.find((i) => i.action === "restart")!.enabled, false, "restart disabled for paused");
assertEqual(pausedItems.find((i) => i.action === "kill")!.enabled, true, "kill enabled for paused");

// ---- destructive flag -------------------------------------------------------

// Only kill is marked as destructive.
assertEqual(allItems.find((i) => i.action === "kill")!.destructive, true, "kill is destructive");
assertEqual(allItems.find((i) => i.action === "pause")!.destructive, false, "pause is not destructive");
assertEqual(allItems.find((i) => i.action === "resume")!.destructive, false, "resume is not destructive");
assertEqual(allItems.find((i) => i.action === "restart")!.destructive, false, "restart is not destructive");

// ---- label casing -----------------------------------------------------------

// All labels are lowercase (matches app aesthetic).
for (const item of allItems) {
  assertEqual(item.label, item.label.toLowerCase(), `${item.action} label is lowercase`);
}

// ---- reduceMenuKey: keyboard navigation reducer -----------------------------

// Escape always closes, regardless of focused index or item count.
assertDeepEqual(
  reduceMenuKey("Escape", 0, 4),
  { type: "close" },
  "Escape closes the menu from first item",
);
assertDeepEqual(
  reduceMenuKey("Escape", 3, 4),
  { type: "close" },
  "Escape closes the menu from last item",
);
assertDeepEqual(
  reduceMenuKey("Escape", -1, 4),
  { type: "close" },
  "Escape closes the menu even when nothing is focused",
);

// ArrowDown moves focus forward, stops at the last item.
assertDeepEqual(
  reduceMenuKey("ArrowDown", 0, 4),
  { type: "move", nextIdx: 1 },
  "ArrowDown moves 0 → 1",
);
assertDeepEqual(
  reduceMenuKey("ArrowDown", 2, 4),
  { type: "move", nextIdx: 3 },
  "ArrowDown moves 2 → 3 (last)",
);
assertDeepEqual(
  reduceMenuKey("ArrowDown", 3, 4),
  { type: "move", nextIdx: 3 },
  "ArrowDown at last item stays clamped (no wrap)",
);

// ArrowUp moves focus backward, stops at the first item.
assertDeepEqual(
  reduceMenuKey("ArrowUp", 3, 4),
  { type: "move", nextIdx: 2 },
  "ArrowUp moves 3 → 2",
);
assertDeepEqual(
  reduceMenuKey("ArrowUp", 1, 4),
  { type: "move", nextIdx: 0 },
  "ArrowUp moves 1 → 0 (first)",
);
assertDeepEqual(
  reduceMenuKey("ArrowUp", 0, 4),
  { type: "move", nextIdx: 0 },
  "ArrowUp at first item stays clamped (no wrap)",
);

// Unrelated keys are no-ops so the popover handler does not preventDefault
// or consume them.
assertDeepEqual(
  reduceMenuKey("Tab", 0, 4),
  { type: "none" },
  "Tab is a no-op (browser handles focus chain)",
);
assertDeepEqual(
  reduceMenuKey("a", 0, 4),
  { type: "none" },
  "Letter keys are no-ops (menu has no typeahead)",
);
assertDeepEqual(
  reduceMenuKey("Enter", 0, 4),
  { type: "none" },
  "Enter is handled at the item level, not the popover (no-op here)",
);

// Empty menu: arrow keys are no-ops because there is nothing to focus.
assertDeepEqual(
  reduceMenuKey("ArrowDown", -1, 0),
  { type: "none" },
  "ArrowDown on empty menu is a no-op",
);
assertDeepEqual(
  reduceMenuKey("ArrowUp", -1, 0),
  { type: "none" },
  "ArrowUp on empty menu is a no-op",
);

// ---- shouldFireAction: activation gate --------------------------------------

const enabledItem: InstanceMenuItem = {
  action: "pause",
  label: "pause",
  enabled: true,
  destructive: false,
};
const disabledItem: InstanceMenuItem = {
  action: "pause",
  label: "pause",
  enabled: false,
  destructive: false,
};

assertEqual(
  shouldFireAction(enabledItem, false),
  true,
  "enabled + not busy fires",
);
assertEqual(
  shouldFireAction(enabledItem, true),
  false,
  "busy blocks an enabled item (prevents double-fire)",
);
assertEqual(
  shouldFireAction(disabledItem, false),
  false,
  "disabled item never fires, even when not busy",
);
assertEqual(
  shouldFireAction(disabledItem, true),
  false,
  "disabled + busy never fires",
);

// ---- routeInstanceAction: kill confirm routing -----------------------------

// Kill must go through the confirm dialog path.
assertDeepEqual(
  routeInstanceAction("kill" as InstanceAction),
  { type: "confirm-kill" },
  "kill routes to confirm dialog",
);

// Every non-kill action must execute immediately with the action preserved.
assertDeepEqual(
  routeInstanceAction("pause" as InstanceAction),
  { type: "immediate", action: "pause" },
  "pause routes to immediate execution",
);
assertDeepEqual(
  routeInstanceAction("resume" as InstanceAction),
  { type: "immediate", action: "resume" },
  "resume routes to immediate execution",
);
assertDeepEqual(
  routeInstanceAction("restart" as InstanceAction),
  { type: "immediate", action: "restart" },
  "restart routes to immediate execution",
);

console.log("InstanceActionsMenu.test.ts ok");
