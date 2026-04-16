#!/usr/bin/env node
/**
 * migrate-content.mjs
 *
 * Migrates the Nextra content tree from src/content/ into docs/ and
 * regenerates sidebars.ts with an explicit docsSidebar from _meta.ts ordering.
 *
 * Usage:
 *   node ./web/docs/scripts/migrate-content.mjs
 *
 * Verify after running:
 *   rg -n '\]\(/' web/docs/docs   # should be empty
 *   cd web/docs && npm run build
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync } from "fs";
import { join, dirname, relative } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const DOCS_ROOT = join(__dirname, "..");
const CONTENT_ROOT = join(DOCS_ROOT, "src/content");
const DEST_ROOT = join(DOCS_ROOT, "docs");

// Nextra-only section stub files that must not be copied as docs.
// They are router-level index wrappers with no real content;
// the real landing pages live in the subdirectory index.mdx files.
const SECTION_STUBS = new Set([
  "getting-started.mdx",
  "guides.mdx",
  "cli-reference.mdx",
]);

// ---------------------------------------------------------------------------
// loadMetaRecord
// ---------------------------------------------------------------------------
/**
 * Parse a Nextra _meta.ts file and return the ordered key list and label map.
 *
 * The files follow a simple pattern:
 *   const meta: MetaRecord = {
 *     key: "label",
 *     "hyphenated-key": "label with spaces",
 *     ...
 *   };
 *
 * @param {string} filePath  Absolute path to the _meta.ts file.
 * @returns {{ keys: string[], labels: Record<string, string> }}
 */
function loadMetaRecord(filePath) {
  const content = readFileSync(filePath, "utf8");
  const keys = [];
  const labels = {};

  // Extract the object literal between the first { and matching }
  const objMatch = content.match(/const meta[^=]*=\s*\{([^}]+)\}/s);
  if (!objMatch) return { keys, labels };

  const objContent = objMatch[1];
  const lineRe = /^\s*["']?([\w-]+)["']?\s*:\s*["']([^"']+)["']/gm;
  let m;
  while ((m = lineRe.exec(objContent)) !== null) {
    keys.push(m[1]);
    labels[m[1]] = m[2];
  }

  return { keys, labels };
}

// ---------------------------------------------------------------------------
// buildRouteMap
// ---------------------------------------------------------------------------
/**
 * Build a map from absolute intra-doc URL path to Docusaurus doc ID.
 *
 * Examples:
 *   "/getting-started"           → "getting-started/index"
 *   "/concepts/lifecycle"        → "concepts/lifecycle"
 *   "/faq"                       → "faq"
 *
 * @param {string} contentRoot  Absolute path to src/content/.
 * @returns {Map<string, string>}
 */
function buildRouteMap(contentRoot) {
  const routeMap = new Map();

  const topMeta = loadMetaRecord(join(contentRoot, "_meta.ts"));

  for (const key of topMeta.keys) {
    if (key === "index") {
      routeMap.set("/", "index");
      continue;
    }

    const subMetaPath = join(contentRoot, key, "_meta.ts");
    if (existsSync(subMetaPath)) {
      // Directory section — root maps to the index doc
      routeMap.set(`/${key}`, `${key}/index`);

      const subMeta = loadMetaRecord(subMetaPath);
      for (const subKey of subMeta.keys) {
        if (subKey === "index") continue;
        routeMap.set(`/${key}/${subKey}`, `${key}/${subKey}`);
      }
    } else {
      // Standalone top-level doc (e.g. faq)
      routeMap.set(`/${key}`, key);
    }
  }

  return routeMap;
}

// ---------------------------------------------------------------------------
// rewriteInternalLinks
// ---------------------------------------------------------------------------
/**
 * Rewrite intra-doc links in a markdown document:
 *
 *   Absolute:  [foo](/concepts/lifecycle#ev) → [foo](../concepts/lifecycle.mdx#ev)
 *   Relative:  [foo](./bar)                  → [foo](./bar.mdx)
 *
 * Only rewrites outside fenced code blocks.
 *
 * @param {string} markdown    Full document text.
 * @param {string} fromDocPath Doc ID (relative to docs/, no extension). E.g. "guides/tui-overview".
 * @param {Map<string, string>} routeMap
 * @returns {string}
 */
function rewriteInternalLinks(markdown, fromDocPath, routeMap) {
  const fromDir = dirname(fromDocPath); // "guides", "daemon", "." for root
  const lines = markdown.split("\n");
  let inFence = false;
  let fenceOpen = "";

  const processed = lines.map((line) => {
    // Track fenced code block boundaries
    const fenceMatch = line.match(/^(`{3,}|~{3,})/);
    if (fenceMatch) {
      if (!inFence) {
        inFence = true;
        fenceOpen = fenceMatch[1];
      } else if (
        line.trimEnd() === fenceOpen ||
        line.startsWith(fenceOpen) && line.trim().length === fenceOpen.length
      ) {
        inFence = false;
        fenceOpen = "";
      }
      return line;
    }
    if (inFence) return line;

    // -----------------------------------------------------------------------
    // 1. Absolute intra-doc links: ](/ ... )
    // -----------------------------------------------------------------------
    line = line.replace(
      /\]\((\/[^)#\s]*?)(#[^)]*?)?\)/g,
      (match, urlPath, fragment) => {
        const targetDoc = routeMap.get(urlPath);
        if (!targetDoc) return match; // unknown route — leave untouched

        let rel = relative(fromDir, targetDoc).replace(/\\/g, "/");
        if (!rel.startsWith(".")) rel = "./" + rel;

        return `](${rel}.mdx${fragment || ""})`;
      }
    );

    // -----------------------------------------------------------------------
    // 2. Relative links without a file extension: ](./foo) or ](../foo)
    // -----------------------------------------------------------------------
    line = line.replace(
      /\]\((\.\.?\/[^)#\s]*?)(#[^)]*?)?\)/g,
      (match, relPath, fragment) => {
        // Skip if path already carries a file extension
        if (/\.[a-z]+$/.test(relPath)) return match;
        return `](${relPath}.mdx${fragment || ""})`;
      }
    );

    return line;
  });

  return processed.join("\n");
}

// ---------------------------------------------------------------------------
// emitDoc
// ---------------------------------------------------------------------------
/**
 * Read a source MDX file, add/update frontmatter (title + optional slug),
 * rewrite internal links, and write to the destination path.
 *
 * @param {string} sourcePath       Absolute path to the source .mdx file.
 * @param {string} destinationPath  Absolute path to the destination .mdx file.
 * @param {Map<string, string>} routeMap
 */
function emitDoc(sourcePath, destinationPath, routeMap) {
  let raw = readFileSync(sourcePath, "utf8");

  // Strip existing YAML frontmatter if present so we can rebuild it cleanly
  let body = raw;
  if (raw.startsWith("---")) {
    const end = raw.indexOf("\n---", 3);
    if (end !== -1) {
      body = raw.slice(end + 4).replace(/^\n/, "");
    }
  }

  // Extract H1 title
  const h1Match = body.match(/^#\s+(.+)$/m);
  const title = h1Match ? h1Match[1].trim() : "untitled";

  // Doc ID relative to docs/
  const docId = relative(DEST_ROOT, destinationPath).replace(/\.mdx$/, "").replace(/\\/g, "/");

  // Rewrite links in the body
  const rewrittenBody = rewriteInternalLinks(body, docId, routeMap);

  // Build frontmatter
  const fmLines = ["---", `title: "${title.replace(/"/g, '\\"')}"`];
  if (docId === "index") {
    fmLines.push("slug: /");
  }
  fmLines.push("---");

  const finalContent = fmLines.join("\n") + "\n\n" + rewrittenBody;

  mkdirSync(dirname(destinationPath), { recursive: true });
  writeFileSync(destinationPath, finalContent, "utf8");
  console.log(`  ✓ ${docId}`);
}

// ---------------------------------------------------------------------------
// buildSidebarContent
// ---------------------------------------------------------------------------
/**
 * Build a sidebar item (string or category object) for a top-level section.
 *
 * @param {string}   sectionKey    Top-level key from _meta.ts (e.g. "concepts").
 * @param {string}   label         Display label (lowercase, e.g. "concepts").
 * @param {{ keys: string[], labels: Record<string, string> } | null} subMeta
 *   Null for standalone docs (faq, index).
 * @returns {string | object}
 */
function buildSidebarContent(sectionKey, label, subMeta) {
  if (!subMeta) {
    return sectionKey; // standalone doc — just the id string
  }

  return {
    type: "category",
    label,
    link: { type: "doc", id: `${sectionKey}/index` },
    items: subMeta.keys.filter((k) => k !== "index").map((k) => `${sectionKey}/${k}`),
  };
}

// ---------------------------------------------------------------------------
// Sidebar TypeScript serializer
// ---------------------------------------------------------------------------
function serializeSidebarItem(item, indent) {
  if (typeof item === "string") {
    return `${indent}"${item}"`;
  }

  const itemsLines = item.items.map((id) => `${indent}    "${id}",`).join("\n");
  return [
    `${indent}{`,
    `${indent}  type: "category",`,
    `${indent}  label: "${item.label}",`,
    `${indent}  link: { type: "doc", id: "${item.link.id}" },`,
    `${indent}  items: [`,
    itemsLines,
    `${indent}  ],`,
    `${indent}}`,
  ].join("\n");
}

function writeSidebarTs(items) {
  const body = items.map((item) => serializeSidebarItem(item, "    ")).join(",\n");
  const ts = `import type { SidebarsConfig } from "@docusaurus/plugin-content-docs";

const sidebars: SidebarsConfig = {
  docsSidebar: [
${body},
  ],
};

export default sidebars;
`;
  writeFileSync(join(DOCS_ROOT, "sidebars.ts"), ts, "utf8");
  console.log("  ✓ sidebars.ts");
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------
async function main() {
  console.log("=== migrate-content ===\n");

  // 1. Build route map
  console.log("Building route map...");
  const routeMap = buildRouteMap(CONTENT_ROOT);
  console.log(`  ${routeMap.size} routes mapped\n`);

  // 2. Migrate docs
  console.log("Migrating docs...");

  const topMeta = loadMetaRecord(join(CONTENT_ROOT, "_meta.ts"));
  const sidebarItems = [];

  for (const key of topMeta.keys) {
    const subMetaPath = join(CONTENT_ROOT, key, "_meta.ts");

    if (key === "index") {
      emitDoc(
        join(CONTENT_ROOT, "index.mdx"),
        join(DEST_ROOT, "index.mdx"),
        routeMap
      );
      sidebarItems.push("index");
      continue;
    }

    if (existsSync(subMetaPath)) {
      // Directory section
      const subMeta = loadMetaRecord(subMetaPath);

      for (const subKey of subMeta.keys) {
        const srcFile =
          subKey === "index"
            ? join(CONTENT_ROOT, key, "index.mdx")
            : join(CONTENT_ROOT, key, subKey + ".mdx");
        const destFile = join(DEST_ROOT, key, subKey + ".mdx");
        if (existsSync(srcFile)) {
          emitDoc(srcFile, destFile, routeMap);
        } else {
          console.warn(`  ⚠ missing source: ${srcFile}`);
        }
      }

      sidebarItems.push(buildSidebarContent(key, topMeta.labels[key], subMeta));
    } else {
      // Standalone top-level doc (faq, etc.)
      // Skip section stubs — they have no real content
      const stubFile = key + ".mdx";
      if (SECTION_STUBS.has(stubFile)) {
        console.log(`  (skipped stub: ${stubFile})`);
        continue;
      }

      const srcPath = join(CONTENT_ROOT, key + ".mdx");
      const destPath = join(DEST_ROOT, key + ".mdx");
      if (existsSync(srcPath)) {
        emitDoc(srcPath, destPath, routeMap);
      }
      sidebarItems.push(buildSidebarContent(key, topMeta.labels[key], null));
    }
  }

  // 3. Generate sidebars.ts
  console.log("\nGenerating sidebars.ts...");
  writeSidebarTs(sidebarItems);

  // 4. Verify no absolute links remain
  console.log("\nVerifying links...");
  const { execSync } = await import("child_process");
  const rgResult = execSync(
    `rg -rn ']\\(/' "${DEST_ROOT}" 2>/dev/null || true`,
    { encoding: "utf8" }
  ).trim();

  if (rgResult) {
    console.warn("⚠  Remaining absolute links found:");
    console.warn(rgResult);
    process.exit(1);
  } else {
    console.log("  ✓ No absolute intra-doc links found");
  }

  // 5. Count generated docs
  const countResult = execSync(
    `fd '\\.mdx$' "${DEST_ROOT}" --type f | wc -l`,
    { encoding: "utf8" }
  ).trim();
  console.log(`\n✅ Migration complete — ${countResult.trim()} docs in docs/`);
}

main().catch((err) => {
  console.error("Migration failed:", err);
  process.exit(1);
});
