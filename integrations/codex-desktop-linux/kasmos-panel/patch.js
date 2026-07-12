"use strict";

function unresolvedMountPoint(description) {
  return function applyKasmosPanelMount(source) {
    console.warn(`WARN: ${description} is not implemented; leaving Codex bundle unchanged`);
    return source;
  };
}

module.exports = {
  descriptors: [
    {
      id: "kasmos-panel-sidebar-nav",
      phase: "webview-asset",
      order: 20_900,
      ciPolicy: "optional",
      // TODO(codex-fork): sidebar nav entry
      apply: unresolvedMountPoint("kasmos-panel sidebar nav entry"),
    },
    {
      id: "kasmos-panel-persistent-pane",
      phase: "webview-asset",
      order: 20_901,
      ciPolicy: "optional",
      // TODO(codex-fork): persistent pane container
      apply: unresolvedMountPoint("kasmos-panel persistent pane container"),
    },
  ],
};
