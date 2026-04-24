import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { emitManifest } from "./emit.mjs";

test("emitManifest writes llms.txt and llms-full.txt", async () => {
  // --- set up a fake docs tree ---
  const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), "llms-test-"));
  const docsDir = path.join(tmpDir, "docs");
  const outDir = path.join(tmpDir, "build");
  await fs.mkdir(docsDir, { recursive: true });
  await fs.mkdir(outDir, { recursive: true });

  // index doc
  await fs.writeFile(
    path.join(docsDir, "index.mdx"),
    `---\ntitle: Welcome\ndescription: kasmos overview\n---\n\n# Welcome\n\nThis is the kasmos documentation.\n`
  );

  // configuration subdirectory with a daemon-toml doc
  const configDir = path.join(docsDir, "configuration");
  await fs.mkdir(configDir, { recursive: true });
  await fs.writeFile(
    path.join(configDir, "daemon-toml.mdx"),
    `---\ntitle: daemon.toml reference\n---\n\n## daemon.toml\n\nThe daemon config file.\n`
  );
  await fs.writeFile(
    path.join(configDir, "index.mdx"),
    `---\ntitle: Configuration\n---\n\n# Configuration overview\n`
  );

  await emitManifest({
    outDir,
    siteDir: tmpDir,
    docsDir: "docs",
    versionedDocsDir: "versioned_docs",
    baseUrl: "https://kasmos.kasthe.co/docs/",
  });

  // --- assertions for llms.txt ---
  const llmsTxt = await fs.readFile(path.join(outDir, "llms.txt"), "utf8");
  assert.match(
    llmsTxt,
    /^[a-z0-9/\-]+\t/m,
    "llms.txt should contain at least one slug line matching /^[a-z0-9/-]+\\t/m"
  );
  assert.ok(llmsTxt.includes("configuration/daemon-toml"), "llms.txt should include configuration/daemon-toml");

  // --- assertions for llms-full.txt ---
  const llmsFull = await fs.readFile(path.join(outDir, "llms-full.txt"), "utf8");
  assert.ok(llmsFull.startsWith("## "), "llms-full.txt should start with '## '");
  assert.ok(llmsFull.includes("## index"), "llms-full.txt should contain ## index");
  assert.ok(llmsFull.includes("## configuration/daemon-toml"), "llms-full.txt should contain ## configuration/daemon-toml");

  // --- cleanup ---
  await fs.rm(tmpDir, { recursive: true, force: true });
});
