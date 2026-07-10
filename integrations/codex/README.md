# Kasmos for Codex

This plugin makes Kasmos available to Codex as a guarded, headless implementation backend. It bundles the `coordinate-kasmos` skill and connects Codex to the local Kasmos MCP server for planning, implementation, review, and verification workflows.

## prerequisite

The Kasmos MCP host must be running at `http://127.0.0.1:7434/mcp` before Codex can use the plugin. Start the installed user service:

```sh
systemctl --user start kasmosdb
```

Or run the server from a Kasmos checkout:

```sh
kas serve
```

Use current task commands such as `kas task list` to inspect work managed by the server.

## install

From the Kasmos repository root, add the local marketplace and install the plugin with the Codex CLI:

```bash
codex plugin marketplace add "$(pwd)/integrations/codex"
codex plugin add kasmos@kasthedev
```

Start a new Codex session after installation so the plugin is loaded. The plugin does not require credentials. It registers the bundled local MCP server, which can then be enabled in Codex.

See the [Codex plugin guide](../../web/docs/docs/guides/codex-plugin.mdx) for setup, verification, and troubleshooting.
