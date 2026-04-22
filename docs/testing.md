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

## measured baselines (2026-04-22)

timings collected on the development machine with a warmed module cache.

| scenario | time |
|----------|------|
| cold suite (`go test -count=1 ./...`) | 1:14.21 |
| warm suite (`go test ./...`, second pass) | 3.862s |
| `app` package cold (`go test -count=1 ./app`) | 72.440s |
| `session/tmux` package cold (`go test -count=1 ./session/tmux`) | 36.120s |

`app` and `session/tmux` dominate the cold path. that is expected: both packages
drive real bubbletea update loops and tmux fakes. the warm-cache path is already
healthy; optimisation work targets the cold path and CI.

## reproducible benchmarks

run `./scripts/bench_tests.sh` (or `make bench-tests` / `just bench-tests`) to
get:

1. a warm-cache timing
2. a forced cold-path timing
3. a per-package elapsed table sorted descending

the script uses bash `TIMEFORMAT` / `time` for sub-second precision and `jq` to
parse `go test -json` output. no GNU `time` or external timing tools are needed.

## ci

the build matrix runs `go build ./...` and `go vet ./...`. the test suite runs
separately via `make test-full` so the suite cost is paid once, not once per
matrix leg.
