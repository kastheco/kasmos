---
work_package_id: WP16
title: Release Workflow (just release X.Y.Z)
lane: planned
dependencies:
- WP01
subtasks:
- Replace hardcoded version with ldflags injection
- Rewrite Justfile for Go project (build, test, install, release)
- Create .goreleaser.yaml (linux/amd64, darwin/amd64+arm64, homebrew tap)
- Create kastheco/homebrew-tap repo for brew install
- Version bump in source + goreleaser release in one command
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
- timestamp: '2026-02-18T00:00:00Z'
  lane: planned
  agent: planner
  action: Updated to use goreleaser with homebrew tap
---

# Work Package Prompt: WP16 - Release Workflow (`just release X.Y.Z`)

## Mission

Implement a one-command release workflow: `just release 2.0.1` updates the version
in source, tags, and runs goreleaser to build cross-platform binaries, publish a
GitHub release with artifacts and changelog, and push a Homebrew formula so users
can `brew install kastheco/tap/kasmos`. Also rewrite the Justfile from legacy
Rust/cargo recipes to Go-native commands.

## Scope

### Files to Create

```
.goreleaser.yaml             # goreleaser config (builds, archives, homebrew, changelog)
```

### Files to Modify

```
Justfile                     # Full rewrite: Go build/test/install/release recipes
cmd/kasmos/main.go           # Use ldflags-injected version variable
internal/tui/panels.go       # Use shared version variable instead of hardcoded const
internal/tui/model.go        # Add version field to Model, update NewModel signature
.gitignore                   # Add dist/
```

### External Setup (manual, not automated)

Before the first release, create the Homebrew tap repo:

```sh
gh repo create kastheco/homebrew-tap --public --description "Homebrew formulae for kastheco projects"
```

And ensure a `GH_PAT` environment variable is available with `repo` scope for
goreleaser to push the formula. A fine-grained PAT scoped to `kastheco/homebrew-tap`
with Contents read/write is sufficient.

## Implementation

### Version Injection via ldflags

Currently the version is hardcoded in two places:
- `cmd/kasmos/main.go`: `fmt.Fprintln(cmd.OutOrStdout(), "kasmos v2.0.0")`
- `internal/tui/panels.go`: `const appVersion = "v2.0.0"`

Replace with a single injectable variable in `cmd/kasmos/main.go`:

```go
// Set at build time: -ldflags "-X main.version=2.0.1"
var version = "dev"
```

Then pass it through to the TUI model:

```go
// cmd/kasmos/main.go — in RunE
model := tui.NewModel(backend, source, version)
```

Update `NewModel` signature:

```go
// internal/tui/model.go
func NewModel(backend worker.WorkerBackend, source task.Source, version string) *Model {
    // ...
    m.version = version
    // ...
}
```

Add `version string` field to the Model struct.

In `internal/tui/panels.go`, delete the `appVersion` const and use `m.version`:

```go
func (m *Model) renderHeader() string {
    v := m.version
    if v != "" && v[0] != 'v' {
        v = "v" + v
    }
    version := versionStyle.Render(v)
    // ...
}
```

### .goreleaser.yaml

```yaml
version: 2

env:
  - CGO_ENABLED=0

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - id: kasmos
    main: ./cmd/kasmos
    binary: kasmos
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.CommitDate}}
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: linux
        goarch: arm64

archives:
  - id: default
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format: tar.gz
    files:
      - LICENSE*
      - README.md

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  use: github
  groups:
    - title: Features
      regexp: '^.*feat[\w)]*:.*$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*fix[\w)]*:.*$'
      order: 1
    - title: Tasks
      regexp: '^.*tasks[\w)]*:.*$'
      order: 2
    - title: Others
      order: 999
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "Merge pull request"

release:
  github:
    owner: kastheco
    name: kasmos
  draft: false
  prerelease: auto
  name_template: "kasmos v{{.Version}}"

brews:
  - repository:
      owner: kastheco
      name: homebrew-tap
      token: "{{ .Env.GH_PAT }}"
    directory: Formula
    homepage: "https://github.com/kastheco/kasmos"
    description: "TUI agent orchestrator for concurrent OpenCode sessions"
    license: "MIT"
    install: |
      bin.install "kasmos"
    test: |
      system "#{bin}/kasmos", "--version"
```

Key config decisions:
- **linux/amd64 + darwin/amd64 + darwin/arm64**: no Windows (bubbletea needs Unix
  terminal), no linux/arm64 (uncommon for dev tooling)
- **tar.gz archives**: goreleaser convention, includes README and LICENSE
- **Changelog groups**: matches our commit convention (`feat(016):`, `fix(016):`, `tasks(016):`)
- **Homebrew tap**: pushes formula to `kastheco/homebrew-tap` repo so users get
  `brew install kastheco/tap/kasmos`
- **`-trimpath -s -w`**: reproducible builds, stripped debug symbols (smaller binary)

### Justfile Rewrite

Replace the entire Justfile (currently Rust/cargo recipes) with Go-native recipes:

```just
set shell := ["bash", "-cu"]

# Build kasmos binary
build:
    go build ./cmd/kasmos

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

# Dry-run release (no publish)
release-dry v:
    #!/usr/bin/env bash
    set -euo pipefail
    VERSION="{{v}}"
    echo "==> Dry run for kasmos v${VERSION}"
    goreleaser release --snapshot --clean
    echo "==> Artifacts in dist/"

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

    BRANCH=$(git branch --show-current)
    echo "    branch: ${BRANCH}"

    # 2. Update version in source
    sed -i "s/var version = \".*\"/var version = \"${VERSION}\"/" cmd/kasmos/main.go
    if [[ -n "$(git status --porcelain)" ]]; then
        git add cmd/kasmos/main.go
        git commit -m "release: v${VERSION}"
        echo "    committed version bump"
    fi

    # 3. Tag
    git tag -a "${TAG}" -m "kasmos ${TAG}"
    echo "    tagged ${TAG}"

    # 4. Push commit + tag
    git push origin "${BRANCH}"
    git push origin "${TAG}"
    echo "    pushed to origin"

    # 5. Goreleaser builds, creates GH release, pushes homebrew formula
    goreleaser release --clean
    echo "==> Done: https://github.com/kastheco/kasmos/releases/tag/${TAG}"
```

### .gitignore

Add `dist/` (goreleaser output directory):

```
/kasmos
/dist/
```

### What Users Get

After a release, users can install kasmos via:

**Homebrew (macOS/Linux):**
```sh
brew install kastheco/tap/kasmos
```

**Direct download:**
```sh
# macOS Apple Silicon
curl -Lo kasmos.tar.gz https://github.com/kastheco/kasmos/releases/latest/download/kasmos_X.Y.Z_darwin_arm64.tar.gz
tar xzf kasmos.tar.gz
sudo mv kasmos /usr/local/bin/

# Linux
curl -Lo kasmos.tar.gz https://github.com/kastheco/kasmos/releases/latest/download/kasmos_X.Y.Z_linux_amd64.tar.gz
tar xzf kasmos.tar.gz
sudo mv kasmos /usr/local/bin/
```

**From source:**
```sh
go install github.com/kastheco/kasmos/cmd/kasmos@latest
```

## Dependencies

- `goreleaser` — `go install github.com/goreleaser/goreleaser/v2@latest`
- `gh` — GitHub CLI (already used in project)
- `GH_PAT` env var — GitHub PAT with `repo` scope for homebrew tap push
- `just` — task runner (already used in project)

## What NOT to Do

- Do NOT create a GitHub Actions workflow — releases are manual via `just release`
- Do NOT build Windows artifacts — kasmos requires a Unix terminal
- Do NOT include `commit` or `date` ldflags vars unless you also add them to
  `main.go` (goreleaser sets them but they need corresponding `var` declarations)
- Do NOT use goreleaser Pro features (`--split`, `--merge`) — free tier is sufficient
- Do NOT modify go.mod or go.sum as part of the release recipe

## Acceptance Criteria

1. `just build` compiles kasmos, `just test` runs tests, `just install` installs
2. `just release-dry 2.0.1` builds all artifacts in `dist/` without publishing
3. `just release 2.0.1` with clean tree: bumps version, tags, pushes, goreleaser
   publishes GH release with 3 platform archives + checksums + changelog
4. `just release 2.0.1` with dirty tree: exits with error
5. Built binary reports correct version: `kasmos --version` prints `kasmos v2.0.1`
6. TUI header shows the ldflags-injected version
7. Homebrew formula pushed to `kastheco/homebrew-tap` — `brew install kastheco/tap/kasmos` works
8. `dist/` is in `.gitignore`
9. Old Rust/cargo recipes fully replaced in Justfile
10. `go test ./...` passes
11. `go build ./cmd/kasmos` passes
