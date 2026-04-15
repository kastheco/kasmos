// Pure DOM-free tests for InstanceActionsMenu helper logic.
// Covers action ordering and enabled/disabled state calculation.
// Runs via `tsx` (same pattern as api.test.ts) — no test framework required.

import type { InstanceEntry, InstanceAction } from "../types.ts";
import { getMenuItems } from "./InstanceActionsMenu.tsx";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
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

console.log("InstanceActionsMenu.test.ts ok");
