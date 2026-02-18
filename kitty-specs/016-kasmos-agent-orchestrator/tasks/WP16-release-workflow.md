---
work_package_id: WP16
title: Release Workflow (just release X.Y.Z)
lane: planned
dependencies:
- WP01
subtasks:
- Replace hardcoded version with ldflags injection
- Rewrite Justfile for Go project (build, test, install, release)
- Build matrix (linux/amd64, darwin/amd64, darwin/arm64)
- Git tag + GitHub release + artifact upload via gh CLI
- Version bump in source files before tagging
phase: Wave 4 - Dashboard Enhancements
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-18T00:00:00Z'
  lane: planned
  agent: planner
  action: Specified by user request
---

# Work Package Prompt: WP16 - Release Workflow (`just release X.Y.Z`)

## Mission

Implement a one-command release workflow: `just release 2.0.1` updates the version
in source, builds cross-platform binaries, creates a git tag, publishes a GitHub
release, and attaches the artifacts. Also rewrite the Justfile from the legacy
Rust/cargo recipes to Go-native commands.

## Scope

### Files to Create

None — all changes are to existing files.

### Files to Modify

```
Justfile                     # Full rewrite: Go build/test/install/release recipes
cmd/kasmos/main.go           # Use ldflags-injected version variable
internal/tui/panels.go       # Use shared version variable instead of hardcoded const
```

### Possible New File

```
version.go                   # Single source of truth for version (root package or internal/)
```

## Implementation

### Version Injection via ldflags

Currently the version is hardcoded in two places:
- `cmd/kasmos/main.go`: `fmt.Fprintln(cmd.OutOrStdout(), "kasmos v2.0.0")`
- `internal/tui/panels.go`: `const appVersion = "v2.0.0"`

Replace with a single injectable variable. Create a minimal version package or
use a package-level var in `cmd/kasmos/`:

**Option A — version var in main (simplest)**:

In `cmd/kasmos/main.go`:
```go
// Set via ldflags: -ldflags "-X main.version=2.0.1"
var version = "dev"
```

Then reference `version` in the `--version` flag handler and pass it to the TUI
model. Add a `Version` field or param to `NewModel` so `panels.go` can read it
instead of a hardcoded const.

The `appVersion` const in `panels.go` becomes a field on Model:
```go
// In model.go
type Model struct {
    // ...
    version string
}

// In NewModel
func NewModel(backend worker.WorkerBackend, source task.Source, version string) *Model {
    // ...
    m.version = version
    // ...
}
```

And `panels.go` uses `m.version` instead of `appVersion`.

Build command becomes:
```
go build -ldflags "-X main.version=2.0.1" ./cmd/kasmos
```

### Justfile Rewrite

Replace the entire Justfile (currently Rust/cargo recipes) with Go-native recipes.
Use `set shell := ["bash", "-cu"]` for compatibility with the release script logic.

```just
set shell := ["bash", "-cu"]

version := "dev"

# Build kasmos binary
build:
    go build ./cmd/kasmos

# Build with version
build-version v:
    go build -ldflags "-X main.version={{v}}" -o kasmos ./cmd/kasmos

# Install to GOPATH/bin
install:
    go install ./cmd/kasmos

# Run tests
test:
    go test ./...

# Run linter
lint:
    go vet ./...

# Run kasmos (pass-through args)
run *ARGS:
    go run ./cmd/kasmos {{ARGS}}

# Full release: just release 2.0.1
release v:
    #!/usr/bin/env bash
    set -euo pipefail

    VERSION="{{v}}"
    TAG="v${VERSION}"

    echo "==> Releasing kasmos ${TAG}"

    # 1. Ensure clean working tree
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "ERROR: working tree is dirty, commit or stash first"
        exit 1
    fi

    # 2. Ensure on main or charm branch
    BRANCH=$(git branch --show-current)
    echo "    branch: ${BRANCH}"

    # 3. Update version in source files
    sed -i "s/var version = \".*\"/var version = \"${VERSION}\"/" cmd/kasmos/main.go
    echo "    updated cmd/kasmos/main.go"

    # 4. Commit version bump if changed
    if [[ -n "$(git status --porcelain)" ]]; then
        git add cmd/kasmos/main.go
        git commit -m "release: v${VERSION}"
        echo "    committed version bump"
    fi

    # 5. Build artifacts
    echo "==> Building artifacts"
    mkdir -p dist

    GOOS=linux  GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" \
        -o "dist/kasmos-${TAG}-linux-amd64" ./cmd/kasmos
    echo "    built linux/amd64"

    GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" \
        -o "dist/kasmos-${TAG}-darwin-amd64" ./cmd/kasmos
    echo "    built darwin/amd64"

    GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" \
        -o "dist/kasmos-${TAG}-darwin-arm64" ./cmd/kasmos
    echo "    built darwin/arm64"

    # 6. Generate checksums
    cd dist
    sha256sum kasmos-${TAG}-* > kasmos-${TAG}-checksums.txt
    cd ..
    echo "    generated checksums"

    # 7. Create git tag
    git tag -a "${TAG}" -m "kasmos ${TAG}"
    echo "    tagged ${TAG}"

    # 8. Push commit + tag
    git push origin "${BRANCH}"
    git push origin "${TAG}"
    echo "    pushed to origin"

    # 9. Create GitHub release with artifacts
    gh release create "${TAG}" \
        --title "kasmos ${TAG}" \
        --generate-notes \
        dist/kasmos-${TAG}-linux-amd64 \
        dist/kasmos-${TAG}-darwin-amd64 \
        dist/kasmos-${TAG}-darwin-arm64 \
        dist/kasmos-${TAG}-checksums.txt
    echo "    created GitHub release"

    # 10. Cleanup
    rm -rf dist
    echo "==> Done: https://github.com/kastheco/kasmos/releases/tag/${TAG}"
```

### .gitignore

Add `dist/` to `.gitignore` so build artifacts aren't accidentally committed:

```
/kasmos
/dist/
```

### Release Artifact Naming

Artifacts follow the standard convention:
```
kasmos-v2.0.1-linux-amd64
kasmos-v2.0.1-darwin-amd64
kasmos-v2.0.1-darwin-arm64
kasmos-v2.0.1-checksums.txt
```

No `.tar.gz` wrapping — ship raw binaries since kasmos is a single static binary
with zero runtime dependencies. Users download and `chmod +x`.

The checksums file uses `sha256sum` format:
```
abc123...  kasmos-v2.0.1-linux-amd64
def456...  kasmos-v2.0.1-darwin-amd64
789abc...  kasmos-v2.0.1-darwin-arm64
```

### GitHub Release Notes

`gh release create` with `--generate-notes` auto-generates notes from commits
since the previous tag. Since this is the first release, it will include all
commits on the branch.

For subsequent releases, the auto-generated notes will show the diff between tags.

## What NOT to Do

- Do NOT use goreleaser — it's overkill for 3 build targets and adds a dependency
- Do NOT build Windows artifacts — kasmos requires a Unix terminal (bubbletea)
- Do NOT create a GitHub Actions workflow — releases are manual via `just release`
- Do NOT strip binaries (`-s -w` ldflags) unless size becomes a concern
- Do NOT create `.tar.gz` archives — ship raw binaries
- Do NOT modify go.mod or go.sum as part of the release

## Acceptance Criteria

1. `just build` compiles kasmos, `just test` runs tests, `just install` installs
2. `just release 2.0.1` with clean tree: bumps version in source, builds 3 binaries,
   creates tag `v2.0.1`, pushes, creates GH release with artifacts attached
3. `just release 2.0.1` with dirty tree: exits with error
4. Built binary reports correct version: `./dist/kasmos-v2.0.1-linux-amd64 --version`
   prints `kasmos v2.0.1`
5. TUI header shows the ldflags-injected version
6. `dist/` is in `.gitignore`
7. Checksums file is correct and attached to release
8. Old Rust/cargo recipes are fully replaced
9. `go test ./...` passes
10. `go build ./cmd/kasmos` passes
