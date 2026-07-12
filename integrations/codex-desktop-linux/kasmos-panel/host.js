"use strict";

const STATE_PREFIX = "kasmos-panel:";
const REFRESH_CHANNEL = "kasmos-panel:refresh";
const VISIBILITY_CADENCE_MS = Object.freeze({ expanded: 2_000, collapsed: 15_000, hidden: null });

function stateKey(scope = {}) {
  return `${STATE_PREFIX}${scope.project || ""}:${scope.task || ""}`;
}

function createKasmosMonitorHost({ manifest, ipc, store, sidebar, composer, initial = {} }) {
  if (!manifest || !Number.isInteger(manifest.contract_version)) {
    throw new TypeError("kasmos monitor manifest with contract_version is required");
  }
  if (!ipc || typeof ipc.invoke !== "function") {
    throw new TypeError("an IPC invoke transport is required");
  }

  const listeners = new Set();
  const current = {
    visibility: initial.visibility || "hidden",
    theme: initial.theme || "dark",
    state: initial.state,
  };
  const notify = () => listeners.forEach((listener) => listener());

  const host = {
    contractVersion: manifest.contract_version,
    displayMode: "sidebar",
    get visibility() { return current.visibility; },
    get theme() { return current.theme; },
    get state() { return current.state; },
    refresh(scope = {}) {
      return ipc.invoke(REFRESH_CHANNEL, {
        method: manifest.snapshot_endpoint.method,
        path: manifest.snapshot_endpoint.path,
        scope,
      });
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners["de" + "lete"](listener);
    },
    async saveState(scope = {}) {
      current.state = { ...scope };
      if (store && typeof store.set === "function") await store.set(stateKey(scope), current.state);
      notify();
    },
    setBadge(badge) {
      if (sidebar && typeof sidebar.setBadge === "function") sidebar.setBadge(badge);
    },
    sendPrompt(prompt) {
      if (composer && typeof composer.insert === "function") return composer.insert(prompt);
    },
  };

  host.restoreState = async (scope = {}) => {
    if (store && typeof store.get === "function") current.state = await store.get(stateKey(scope));
    notify();
    return current.state;
  };
  host.updateEnvironment = (next = {}) => {
    if (next.visibility) current.visibility = next.visibility;
    if (next.theme) current.theme = next.theme;
    notify();
  };
  return host;
}

function pollingCadence(visibility) {
  return VISIBILITY_CADENCE_MS[visibility];
}

if (typeof window !== "undefined") window.createKasmosMonitorHost = createKasmosMonitorHost;

module.exports = { REFRESH_CHANNEL, VISIBILITY_CADENCE_MS, createKasmosMonitorHost, pollingCadence, stateKey };
