set shell := ["bash", "-cu"]
set dotenv-load := true

# Build kasmos binary
build:
    go build -o kasmos .

# Install to GOPATH/bin (with kas, kms aliases)
install:
    go install .
    ln -sf "$(go env GOPATH)/bin/kasmos" "$(go env GOPATH)/bin/kas"
    ln -sf "$(go env GOPATH)/bin/kasmos" "$(go env GOPATH)/bin/kms"

# Build + install
bi: build install

# run with no args
bin:
    kas

setup:
    kas setup --force

# Build + install + run
kas: build install bin

# Alias for kas
kms: kas

# Run tests
test:
    go test ./...

# Run linter
lint:
    go vet ./...

# Run kasmos (pass-through args)
run *ARGS:
    go run . {{ARGS}}

# Tag and push a release (CI runs goreleaser): just release 1.0.0
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
    sd 'version\s*=\s*"[^"]*"' "version     = \"${VERSION}\"" main.go
    if [[ -n "$(git status --porcelain)" ]]; then
        git add main.go
        git commit -m "release: v${VERSION}"
        echo "    committed version bump"
    fi

    # 3. Tag
    git tag -a "${TAG}" -m "kasmos ${TAG}"
    echo "    tagged ${TAG}"

    # 4. Push commit + tag — CI takes it from here
    git push origin "${BRANCH}"
    git push origin "${TAG}"
    echo "==> Pushed ${TAG}. CI will build and publish the release."
    echo "    https://github.com/kastheco/kasmos/releases/tag/${TAG}"

# Clean build artifacts
clean:
    rm -f kasmos
    rm -rf dist/
