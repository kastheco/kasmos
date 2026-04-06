# Optimize MCP Tools Implementation Plan

**Goal:** Make kasmos MCP tools (grep, read_file, find_files, list_dir, git_*) transparently faster and code-aware by adding a caching layer with filesystem-watch invalidation and ctags-based symbol enrichment — without changing any tool signatures or agent-facing behavior.

**Architecture:** A new `internal/mcpserver/cache` package provides a `CachedRunner` decorator (wrapping the existing `CmdRunner` interface) backed by ristretto for memory-bounded LRU caching, and an fsnotify-based `Watcher` that invalidates cache entries when files change. A separate `internal/mcpserver/symbols` package runs universal-ctags in the background on changed files to maintain a per-file symbol index. Grep results are enriched post-hoc with symbol metadata. A new `symbols` tool exposes the index directly. All existing tool signatures, parameter schemas, and response formats remain identical.

**Tech Stack:** Go 1.24+, `github.com/dgraph-io/ristretto` (cache), `github.com/fsnotify/fsnotify` (file watching), `universal-ctags` (symbol extraction via JSON output), existing `mcp-go` server framework.

**Size:** Medium (estimated ~6 hours, 10 tasks, 3 waves)

---

## what this changes

- **Repeated queries become instant.** Agents frequently grep the same pattern, list the same directory, or re-read a file they just read. The caching layer serves these from memory instead of re-spawning `rg`/`fd`/`git` subprocesses.
- **File reads are mtime-aware.** `read_file` checks file modification time before re-reading; unchanged files return cached content.
- **Grep results gain code awareness.** When a grep match lands on a known symbol (function, type, method, variable), the result is annotated with `symbol_kind`, `symbol_name`, and `symbol_parent` — helping agents distinguish definitions from call sites without a follow-up read.
- **New `symbols` tool.** Agents can request a file's symbol outline (names, kinds, lines, signatures) directly, eliminating the grep-then-read pattern for understanding file structure.
- **All changes are transparent.** Existing tool names, parameters, and base response formats are unchanged. Agents that ignore the new fields work identically.

## acceptance criteria

- [ ] grep, find_files, list_dir, and git_* tools return cached results when called with identical parameters and no underlying files have changed, verified by test asserting the subprocess is not re-invoked.
- [ ] modifying a file within the watched workspace invalidates the relevant cache entries within 200ms, verified by test using a mock watcher.
- [ ] read_file returns cached content when file mtime is unchanged; returns fresh content after modification — verified by test.
- [ ] cache memory usage is bounded (configurable max, default 64 MB) and evicts LRU entries when full — verified by ristretto configuration test.
- [ ] grep results for matches on ctags-indexed symbols include `symbol_kind` and `symbol_name` fields — verified by test with a known Go file.
- [ ] the `symbols` tool returns a JSON array of `{name, kind, line, parent, signature}` for a given file path — verified by test.
- [ ] all existing grep/read_file/find_files/list_dir tests continue to pass without modification (backward compatibility).
- [ ] cache can be disabled via `KAS_MCP_NOCACHE=1` environment variable for debugging — verified by test.

## non-goals

- [ ] semantic/embedding-based search (RAG) — out of scope, no vector DB.
- [ ] cross-file reference graph or call-graph DAG — ctags provides per-file symbols only; dependency analysis is deferred.
- [ ] replacing rg/fd/git with pure-Go implementations — we cache subprocess results, not reimplement them.
- [ ] caching across server restarts (persistent on-disk cache) — in-memory only; cache warms naturally within a session.
- [ ] tree-sitter integration — ctags is simpler, covers more languages, and has no CGo dependency.

## assumptions

- [ ] `universal-ctags` binary is available on PATH (already standard on dev machines; installer can add it).
- [ ] fsnotify recursive watching is sufficient for typical workspace sizes (< 50k files). If inotify limits are hit, graceful degradation to no-watch mode is acceptable.
- [ ] ristretto's TinyLFU admission policy handles the mixed workload (frequent grep repeats + one-off reads) well without tuning.
- [ ] agents do not depend on exact byte-for-byte output from tools (they parse structured JSON), so adding optional fields to grep results is non-breaking.

## constraints and risks

- **inotify limits on Linux:** Large workspaces may exhaust the default 8192 watch limit. Mitigation: detect the error, log a warning, fall back to no-watch (cache with short TTL instead).
- **Cache coherency during git operations:** `git checkout`, `git rebase`, etc. change many files at once. The fsnotify watcher must handle burst invalidation without races. Mitigation: debounce with a 100ms window; full-cache flush on git ref change.
- **ctags binary availability:** If ctags is not installed, symbol enrichment must degrade gracefully (grep works normally, symbols tool returns an error). The system must never fail to start due to missing ctags.
- **Memory pressure:** Ristretto's 64 MB default may be too much for constrained environments. Mitigation: configurable via environment variable `KAS_MCP_CACHE_MB`.

## open questions

- Should git_status/git_diff be cached at all? They reflect mutable working-tree state that changes on every edit. Current plan: short TTL (2s) for git tools, instant invalidation on fsnotify events. May need tuning.
- Should the symbols tool support multiple files or directories in one call? Starting with single-file; can extend later based on agent usage patterns.

---

## Wave 1: Cache Infrastructure

Foundation layer — cache store, file watcher, and the CachedRunner decorator. No tool behavior changes yet; all existing tests must still pass.

### Task 1: Cache store package

**Files:**
- Create: `internal/mcpserver/cache/cache.go`
- Create: `internal/mcpserver/cache/cache_test.go`

Implement `cache.Store` wrapping ristretto with:
- `Get(key string) ([]byte, bool)` — retrieve cached bytes
- `Set(key string, value []byte, cost int64)` — store with cost = len(value)
- `Invalidate(key string)` — delete a single key
- `InvalidatePrefix(prefix string)` — delete all keys with a given prefix (for path-based invalidation)
- `Flush()` — clear all entries
- `Close()` — shutdown
- Constructor `NewStore(maxMB int)` with `KAS_MCP_CACHE_MB` env override
- Constructor returns a no-op store when `KAS_MCP_NOCACHE=1`

Test: set/get round-trip, invalidation, prefix invalidation, no-op mode.

### Task 2: File watcher with debouncing

**Files:**
- Create: `internal/mcpserver/cache/watcher.go`
- Create: `internal/mcpserver/cache/watcher_test.go`

Implement `cache.Watcher` using fsnotify:
- Watches a root directory recursively (add subdirectories on create)
- Debounces rapid events (100ms window) into batched change sets
- Emits `ChangeSet{Created, Modified, Deleted []string}` on a channel
- Graceful degradation: if fsnotify setup fails (inotify limit), log warning and return a no-op watcher
- `Stop()` for clean shutdown

Test: mock fsnotify events, verify debouncing collapses rapid writes, verify new subdirectories are watched.

### Task 3: CachedRunner decorator

**Files:**
- Create: `internal/mcpserver/cache/runner.go`
- Create: `internal/mcpserver/cache/runner_test.go`

Implement `cache.CachedRunner` satisfying `fstools.CmdRunner`:
- Wraps an inner `CmdRunner`
- Cache key: SHA-256 of `(binary + args joined)`
- On `Output()`: check cache first; on miss, delegate to inner runner, cache result
- Listens on watcher's change channel to invalidate affected keys
- Key prefix scheme: `grep:<path-hash>:*` for grep results, `fd:<path-hash>:*` for find/listdir, `git:<repo-hash>:*` for git commands
- Invalidation strategy:
  - File modify/delete → invalidate all `grep:` and `fd:` keys containing that file's parent dir
  - Any change → invalidate all `git:` keys (git state is global)
- Short TTL (2s) for `git` commands as secondary safeguard
- Bypass cache entirely when `KAS_MCP_NOCACHE=1`

Test: verify cache hit avoids inner runner call, verify invalidation on file change triggers re-exec, verify git keys expire.

### Task 4: Wire cache into MCP server initialization

**Files:**
- Modify: `internal/mcpserver/fstools/register.go` — accept optional `CmdRunner` override (or use `cache.CachedRunner` when available)
- Modify: `internal/mcpserver/gittools/register.go` — same pattern
- Modify: `cmd/mcp.go` — create `cache.Store`, `cache.Watcher`, `cache.CachedRunner`, pass to `RegisterTools`
- Modify: `internal/mcpserver/server.go` — add `Closer` interface support so `ServeStdio`/`Handler` can shut down the watcher on exit

Wire the cache infrastructure into the existing registration path. The `RegisterTools` functions gain an optional `...Option` pattern or accept `CmdRunner` directly so that `cmd/mcp.go` can inject the cached runner. Existing tests that construct tools with `ExecRunner` are unaffected.

---

## Wave 2: Read Cache + Result Caching

Per-tool caching optimizations. read_file gets mtime-aware caching. Grep/find/listdir benefit from CachedRunner (wired in wave 1) but read_file needs special handling since it doesn't use CmdRunner.

### Task 5: mtime-aware read_file cache

**Files:**
- Create: `internal/mcpserver/cache/filecache.go`
- Create: `internal/mcpserver/cache/filecache_test.go`
- Modify: `internal/mcpserver/fstools/read.go` — integrate file cache

Implement `cache.FileCache`:
- `Get(path string) (content string, totalLines int, hit bool)` — returns cached content if mtime matches
- `Set(path string, mtime time.Time, content string, totalLines int)`
- Invalidate on watcher events
- Backed by the same ristretto `cache.Store`
- Cache key: `read:<path>:<from>:<lines>`; stored value includes mtime for staleness check

Modify `makeReadFileHandler` to check `FileCache` before calling `readFileLines`. On cache miss or stale mtime, read from disk and update cache.

Test: read same file twice — second call hits cache; modify file — next read gets fresh content.

### Task 6: Cache metrics and diagnostics

**Files:**
- Create: `internal/mcpserver/cache/metrics.go`
- Create: `internal/mcpserver/cache/metrics_test.go`

Simple hit/miss/eviction counters exposed via:
- `Metrics() CacheMetrics` struct with `Hits, Misses, Evictions, BytesUsed int64`
- Log cache stats periodically (every 60s) at debug level
- Expose via a new `cache_stats` MCP tool (optional diagnostic tool, not for agents)

Test: verify counters increment correctly on get/set/evict.

---

## Wave 3: Symbol Intelligence

ctags-based symbol indexing and grep enrichment. Adds code awareness without changing any existing tool signatures.

### Task 7: ctags symbol indexer

**Files:**
- Create: `internal/mcpserver/symbols/indexer.go`
- Create: `internal/mcpserver/symbols/indexer_test.go`

Implement `symbols.Indexer`:
- Runs `ctags --output-format=json --fields=+KSn -f - <file>` to extract symbols
- Parses JSON output into `[]Symbol{Name, Kind, Line, End, Parent, Signature}`
- Indexes on startup by walking tracked files (via `git ls-files`)
- Re-indexes individual files on watcher change events
- Graceful degradation: if `ctags` not on PATH, indexer is a no-op (returns empty results, logs once)
- Thread-safe: `sync.RWMutex` on the symbol map

Test: parse known ctags JSON output, verify symbol extraction for Go functions/types/methods.

### Task 8: Symbol store

**Files:**
- Create: `internal/mcpserver/symbols/store.go`
- Create: `internal/mcpserver/symbols/store_test.go`

Implement `symbols.Store`:
- `map[string][]Symbol` keyed by absolute file path
- `Lookup(path string) []Symbol` — returns all symbols for a file
- `LookupAt(path string, line int) *Symbol` — returns the symbol containing that line (for grep enrichment)
- `Update(path string, symbols []Symbol)` — replace symbols for a file
- `Remove(path string)` — clear on file delete

Test: store/lookup round-trip, LookupAt finds enclosing function, Update replaces old symbols.

### Task 9: Grep result enrichment

**Files:**
- Create: `internal/mcpserver/fstools/enrich.go`
- Create: `internal/mcpserver/fstools/enrich_test.go`
- Modify: `internal/mcpserver/fstools/grep.go` — call enricher post-parse

Extend `GrepMatch` with optional fields:
```go
SymbolKind   string `json:"symbol_kind,omitempty"`
SymbolName   string `json:"symbol_name,omitempty"`
SymbolParent string `json:"symbol_parent,omitempty"`
```

Implement `EnrichMatches(matches []GrepMatch, store *symbols.Store) []GrepMatch`:
- For each match, call `store.LookupAt(match.File, match.Line)`
- If a symbol is found, populate the optional fields
- If symbol store is nil or returns nothing, leave fields empty (backward compatible)

Wire into `makeGrepHandler` after `parseRgJSON` — if a symbol store is available, enrich before returning.

Test: grep matches on a function definition get `symbol_kind: "function"`, matches outside symbols have empty fields, nil store produces unchanged output.

### Task 10: Symbols MCP tool

**Files:**
- Create: `internal/mcpserver/symbols/tool.go`
- Create: `internal/mcpserver/symbols/tool_test.go`
- Modify: `cmd/mcp.go` — register symbols tool

Register a new `symbols` tool:
- **Parameters:** `path` (required, file path)
- **Returns:** `{symbols: [{name, kind, line, parent, signature}], total: int}`
- Validates path via sandbox
- Looks up symbols from the store
- Returns empty array with a hint if ctags unavailable

Wire into `cmd/mcp.go` alongside other tool registrations. Pass the shared `symbols.Store` instance.

Test: call symbols tool on a known Go file, verify function/type/method symbols are returned with correct metadata.
