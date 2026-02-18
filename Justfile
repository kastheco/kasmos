set shell := ["bash", "-cu"]
set dotenv-load := true

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
    echo "==> Dry run for kasmos v{{v}}"
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
    GITHUB_TOKEN="${GH_PAT}" goreleaser release --clean
    echo "==> Done: https://github.com/kastheco/kasmos/releases/tag/${TAG}"
