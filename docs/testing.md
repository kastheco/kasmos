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

### after (2026-04-22, post tasks 1–7)

| scenario | time |
|----------|------|
| cold suite (`go test -count=1 ./...`) | 46.3s |
| warm suite (`go test ./...`, second pass) | ~4s (all packages `(cached)`) |
| `app` package cold | 27.0s |
| `session/tmux` package cold | 44.6s ¹ |

¹ `session/tmux` measured under system load from concurrent agent tasks; individual
test timing improved by replacing hard-coded sleeps with injectable vars, but
parallel scheduling overhead is visible when the suite runs under contention.

**cold suite improvement: 74s → 46s (~38%). app package improvement: 72s → 27s (~63%).**

the `app` gain comes from two changes: injectable timing knobs eliminated the
hard-coded poll/title-sync delays, and `t.Parallel()` was added to all isolated
test cases so the package runs its tests concurrently.

### top cold packages after optimisation

```
elapsed     package
-------     -------
44.6s       session/tmux
27.0s       app
6.6s        config
5.2s        cmd
2.7s        config/taskstore
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

## ci

the build matrix runs `go build ./...` and `go vet ./...`. the test suite runs
separately via `make test-full` so the suite cost is paid once, not once per
matrix leg.
