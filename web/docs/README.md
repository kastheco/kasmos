# docs release snapshots

Release tags cut a Docusaurus docs snapshot automatically.
The `docs-version` job in `.github/workflows/release.yml` runs after GoReleaser.
It calls `scripts/cut-docs-version.sh <version>` with the tag minus `v`.
The helper runs `npm ci` under `web/docs`.
Then it runs `npx docusaurus docs:version <version>`.
If `versions.json` already contains the version, the helper exits cleanly.
For manual backfills, run the helper from the repository root on `main`.
Commit the generated `versioned_docs`, `versioned_sidebars`, and `versions.json` changes.
