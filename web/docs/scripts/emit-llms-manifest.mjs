#!/usr/bin/env node
/**
 * CLI wrapper around the llms-manifest emitter.
 * Runs against the already-built `build/` directory so you can regenerate
 * llms.txt / llms-full.txt without a full Docusaurus rebuild.
 *
 * Usage:
 *   node scripts/emit-llms-manifest.mjs [--out <outDir>] [--site <siteDir>]
 *
 * Defaults:
 *   --out   <siteDir>/build
 *   --site  directory of this script's parent (web/docs)
 */

import { fileURLToPath } from "node:url";
import path from "node:path";
import { emitManifest } from "../plugins/llms-manifest/emit.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const siteDir = path.resolve(__dirname, "..");

// Minimal arg parsing
const args = process.argv.slice(2);
function getFlag(flag) {
  const idx = args.indexOf(flag);
  return idx !== -1 ? args[idx + 1] : undefined;
}

const outDir = getFlag("--out") ?? path.join(siteDir, "build");
const siteDirArg = getFlag("--site") ?? siteDir;

console.log(`emitting llms manifest...`);
console.log(`  siteDir : ${siteDirArg}`);
console.log(`  outDir  : ${outDir}`);

await emitManifest({
  outDir,
  siteDir: siteDirArg,
  docsDir: "docs",
  versionedDocsDir: "versioned_docs",
  baseUrl: "https://kasmos.kasthe.co/docs/",
});

console.log("done — wrote llms.txt and llms-full.txt");
