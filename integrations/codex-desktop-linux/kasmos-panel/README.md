# Kasmos panel feature template

This disabled feature is the handoff artifact for the Codex Desktop Linux fork.

Fork integration checklist:

1. Copy this directory to `linux-features/kasmos-panel/`.
2. Fill the `TODO(codex-fork)` sidebar nav entry descriptor in `patch.js`.
3. Fill the `TODO(codex-fork)` persistent pane container descriptor in `patch.js`.
4. Wire the privileged side of `kasmos-panel:refresh` to the manifest-declared snapshot endpoint. The renderer must use IPC and must not perform network I/O.
5. Run `node --test linux-features/kasmos-panel/host.test.mjs` and the fork's Linux feature tests.
6. Only after the integration works, add `"kasmos-panel"` to `linux-features/features.json` under `enabled`.

The stage hook exports the monitor bundle when `kas` is available and otherwise warns without failing the Codex build.
