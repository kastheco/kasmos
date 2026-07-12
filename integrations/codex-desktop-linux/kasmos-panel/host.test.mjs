import assert from "node:assert/strict";
import test from "node:test";
import hostModule from "./host.js";

const { REFRESH_CHANNEL, createKasmosMonitorHost, pollingCadence } = hostModule;
const snapshot = {
  schema_version: 2, generated_at: "2026-07-11T00:00:00Z", project: "kasmos",
  daemon_running: true, lifecycle: { planning: 0, ready: 0, implementing: 1, reviewing: 0, verifying: 0, total: 1 },
  active_agents: [], attention: [], truncated: {},
};
const manifest = {
  contract_version: 1,
  snapshot_endpoint: { method: "POST", path: "/v1/monitor/snapshot" },
};

test("reference host satisfies the sidebar contract", async () => {
  const calls = [];
  const values = new Map();
  const badges = [];
  const prompts = [];
  const host = createKasmosMonitorHost({
    manifest,
    initial: { visibility: "collapsed" },
    ipc: { invoke: async (...args) => { calls.push(args); return snapshot; } },
    store: { get: async (key) => values.get(key), set: async (key, value) => values.set(key, value) },
    sidebar: { setBadge: (badge) => badges.push(badge) },
    composer: { insert: (prompt) => prompts.push(prompt) },
  });

  assert.equal(host.contractVersion, manifest.contract_version);
  assert.equal(host.displayMode, "sidebar");
  assert.equal(host.visibility, "collapsed");
  assert.equal(pollingCadence(host.visibility), 15_000);
  assert.deepEqual(await host.refresh({ project: "kasmos" }), snapshot);
  assert.deepEqual(calls, [[REFRESH_CHANNEL, { method: "POST", path: "/v1/monitor/snapshot", scope: { project: "kasmos" } }]]);

  await host.saveState({ project: "kasmos", task: "panel" });
  await host.restoreState({ project: "kasmos", task: "panel" });
  assert.deepEqual(host.state, { project: "kasmos", task: "panel" });

  const badge = { level: "running", running_agents: 1, blocked: 0, implementing: 1, reviewing: 0 };
  host.setBadge(badge);
  host.sendPrompt("show the active task");
  assert.deepEqual(badges, [badge]);
  assert.deepEqual(prompts, ["show the active task"]);
});

test("subscriptions observe visibility, theme, and restored state", async () => {
  let changes = 0;
  const host = createKasmosMonitorHost({ manifest, ipc: { invoke: async () => snapshot } });
  const unsubscribe = host.subscribe(() => changes++);
  host.updateEnvironment({ visibility: "expanded", theme: "light" });
  await host.restoreState({ project: "kasmos" });
  unsubscribe();
  host.updateEnvironment({ visibility: "hidden" });
  assert.equal(changes, 2);
});
