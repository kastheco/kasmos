# docs MCP tool

Two MCP tools expose the kasmos wiki to agents:

- `mcp__kasmos__docs_search` - regex/literal search across all kasmos docs.
- `mcp__kasmos__docs_read` - fetch a full doc by slug, path, or URL.

## when to use them

- Coder/reviewer agents: confirm signal names, config keys, cli flags before guessing.
- Planner agents: cite documented patterns instead of re-inventing flows.
- Chat: answer "how does X work?" by grounding the answer in the wiki.

## local vs remote mode

The tools prefer local mode (`rg` over `web/docs/docs/**`) when the kasmos repo is checked out. On downstream projects they fall back to HTTPS fetches of `https://kasmos.kasthe.co/docs/llms.txt` and `llms-full.txt`. Each response includes a `source: "local"|"remote"` field so agents can tell which backend answered.

## pinning to a version

Pass `version: "2.6.0"` (or any entry in `web/docs/versions.json`) to restrict results to a historical snapshot. Local mode reads from `web/docs/versioned_docs/version-2.6.0/`; remote mode fetches `https://kasmos.kasthe.co/docs/2.6.0/llms-full.txt`.

## opting out

Set `KASMOS_DOCS_BASE_URL=""` to disable remote mode entirely. Local mode is always available when `web/docs/docs/` is on disk.

## troubleshooting

- "no matches" in local mode but content is present: ensure `rg` is installed (same assumption as `mcp__kasmos__grep`).
- HTTP 5xx in remote mode: the tool surfaces the error verbatim and does not auto-retry.
- Slug not found in `docs_read`: try `docs_search` first to discover the correct slug; MDX filenames sometimes use `index.mdx` in a directory (surface as the directory slug).
