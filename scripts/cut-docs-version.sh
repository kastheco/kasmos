#!/usr/bin/env bash
set -euo pipefail
TAG_VERSION="${1:?usage: cut-docs-version.sh <X.Y.Z>}"

cd web/docs
# Idempotency: no-op if the version is already snapshotted.
if jq -e --arg v "$TAG_VERSION" 'index($v) != null' versions.json >/dev/null; then
  echo "version ${TAG_VERSION} already present in versions.json, skipping"
  exit 0
fi

npm ci
npx docusaurus docs:version "$TAG_VERSION"
cd ../..
git add web/docs/versioned_docs web/docs/versioned_sidebars web/docs/versions.json
