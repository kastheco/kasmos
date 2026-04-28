# testing

## quick reference

| target | command | when to use |
|--------|---------|-------------|
| `test-fast` | `go test ./...` | daily dev — hits the warm cache |
| `test-full` | `go test -count=1 ./...` | pre-push, CI — forces a real run |
| `test-race` | `go test -race ./app ./session/tmux` | verify concurrency in hot packages |
| `bench-tests` | `./scripts/bench_tests.sh` | measure cold/warm suite cost |

run via make or just:

```
make test-fast      just test-fast
make test-full      just test-full
make test-race      just test-race
make bench-tests    just bench-tests
```

## warm-cache path

`test-fast` (`go test ./...`) is the default dev workflow. the go test cache is
repo-wide and survives across shells. after a single run, every unchanged package
is `(cached)` and the full suite returns in seconds.

**`-short` is intentionally not the default.** the codebase currently has no
meaningful `testing.Short()` usage, so passing `-short` would silently skip
nothing useful while hiding real test coverage gaps.

## measured baselines

timings collected on the development machine with a warmed module cache.

### before (2026-04-22, pre-optimisation)

| scenario | time |
|----------|------|
| cold suite (`go test -count=1 ./...`) | 1:14.21 |
| warm suite (`go test ./...`, second pass) | 3.862s |
| `app` package cold | 72.440s |
| `session/tmux` package cold | 36.120s |

### after (2026-04-22, post tasks 1–8 + seam fixes)

| scenario | time |
|----------|------|
| cold suite (`go test -count=1 ./...`) | ~21s |
| warm suite (`go test ./...`, second pass) | ~4s (all packages `(cached)`) |
| `app` package cold | ~21s |
| `session/tmux` package cold | ~0.6s |

**cold suite improvement: 74s → 21s (~72%). session/tmux improvement: 36s → 0.6s (~98%).**

the `session/tmux` gain comes from adding `programReadyMaxWaitTime` and
`codexGracePeriod` overrides to `withFastTmuxTimings`, eliminating the 30s
adapter timeout and 2s codex grace period from all mocked tests. the `app` gain
comes from injectable timing knobs and `t.Parallel()` on all isolated test cases.

### top cold packages after optimisation

```
elapsed     package
-------     -------
20.7s       app
5.3s        config
4.2s        cmd
2.1s        config/taskstore
0.6s        session/tmux
```

## reproducible benchmarks

run `./scripts/bench_tests.sh` (or `make bench-tests` / `just bench-tests`) to
get:

1. a warm-cache timing
2. a forced cold-path timing
3. a per-package elapsed table sorted descending

the script uses bash `TIMEFORMAT` / `time` for sub-second precision and `jq` to
parse `go test -json` output. no GNU `time` or external timing tools are needed.

## parallelism convention

tests are parallel by default in isolated packages:

```go
func TestFoo(t *testing.T) {
    t.Parallel()
    // ...
}
```

a test that must **not** run in parallel carries a comment on the first line of
the test body explaining why:

```go
func TestBar(t *testing.T) {
    // serial: mutates pkgGlobal timing seam shared across tests
    // ...
}
```

the comment pattern is `// serial: <reason>`. the reason should name the specific
shared state (a package-level var, a file, an env var) that makes parallelism
unsafe — not just say "not parallel".

## regression checks

when changing test structure in `app` or `session/tmux`, run:

```
make bench-tests
```

and compare the `app` and `session/tmux` cold times against the baselines above.
a regression of more than ~10s in either package warrants investigation before
merging.

### tmux pty handles

- every real `PtyFactory.Start(cmd)` result must retain ownership of both the
  pty file and the started `exec.Cmd`.
- `Restore()` stays monitor-only; detached preview/status uses `capture-pane`,
  not a background attach client.
- active `Attach()` owns one attach handle, and `Detach()` must close/reap it.
- fake factories return fake handles and must not call real `cmd.Start()`.
- developer regression command: `go test -tags integration_tmux ./session/tmux/...`.

## ci

ci runs one `test` job (`go test ./...`) and four `build` jobs (linux/darwin ×
amd64/arm64). the build jobs declare `needs: test`, so the suite cost is paid
once, not once per matrix leg. the build jobs only run `go build`; they do not
re-run tests.
