import fs from "node:fs/promises";
import path from "node:path";
import matter from "gray-matter";

const MAX_FULL_SIZE = 2_097_152; // 2 MiB

/**
 * Walk a directory recursively, yielding absolute paths for .md / .mdx files.
 * @param {string} dir
 * @returns {Promise<string[]>}
 */
async function walkDocs(dir) {
  const results = [];
  let entries;
  try {
    entries = await fs.readdir(dir, { withFileTypes: true });
  } catch {
    return results;
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results.push(...(await walkDocs(full)));
    } else if (entry.isFile() && /\.(mdx?)$/.test(entry.name)) {
      results.push(full);
    }
  }
  return results;
}

/**
 * Derive a URL slug from a file path relative to the docs root.
 * e.g. "configuration/index.mdx" -> "configuration/"
 *      "getting-started.md"      -> "getting-started"
 * @param {string} relPath  path relative to siteDir/docsDir (no leading slash)
 */
function slugFromRelPath(relPath) {
  // Strip extension
  let slug = relPath.replace(/\.(mdx?)$/, "");
  // index files collapse to their parent dir (with trailing slash)
  if (slug === "index") return "";
  if (slug.endsWith("/index")) return slug.slice(0, -"index".length);
  if (slug.endsWith(path.sep + "index")) {
    slug = slug.slice(0, -(path.sep.length + "index".length)) + "/";
  }
  // Normalise path separators to forward slashes
  return slug.replace(/\\/g, "/");
}

/**
 * Strip frontmatter and MDX-specific syntax from raw file content.
 * @param {string} content  raw file content (frontmatter already stripped by gray-matter)
 */
function stripMdx(content) {
  // Remove import lines
  let out = content.replace(/^import .+?$/gm, "");
  // Remove JSX component blocks (non-greedy, dotAll)
  out = out.replace(/<([A-Z][A-Za-z0-9]*)[^>]*\/?>(?:.*?<\/\1>)?/gs, "");
  // Collapse multiple blank lines
  out = out.replace(/\n{3,}/g, "\n\n").trim();
  return out;
}

/**
 * @typedef {Object} DocEntry
 * @property {string} slug
 * @property {string} title
 * @property {string} description
 * @property {string} section
 * @property {string} url
 * @property {string} version
 * @property {string} body  stripped markdown body
 */

/**
 * Parse a single doc file and return a DocEntry (or null on error).
 * @param {string} filePath      absolute path to the file
 * @param {string} docsRoot      absolute path to the docs root (siteDir/docsDir)
 * @param {string} baseUrl       e.g. "https://kasmos.kasthe.co/docs/"
 * @param {string} version       "current" | "2.5.0" etc.
 */
async function parseDoc(filePath, docsRoot, baseUrl, version) {
  const raw = await fs.readFile(filePath, "utf8");
  const { data, content } = matter(raw);

  const relPath = path.relative(docsRoot, filePath);
  const slug = slugFromRelPath(relPath);
  const section = slug.split("/")[0] || "root";
  const url = baseUrl.replace(/\/$/, "") + "/" + slug;
  const title = (data.title || slug || "untitled").toString();
  const description = (data.description || "").toString();
  const body = stripMdx(content);

  return { slug, title, description, section, url, version, body };
}

/**
 * Main emitter — called by the Docusaurus plugin and CLI script.
 *
 * @param {object} opts
 * @param {string} opts.outDir              absolute path to build output dir
 * @param {string} opts.siteDir             absolute path to siteDir (docusaurus root)
 * @param {string} opts.docsDir             relative path from siteDir (e.g. "docs")
 * @param {string} opts.versionedDocsDir    relative path from siteDir (e.g. "versioned_docs")
 * @param {string} opts.baseUrl             full base URL incl. trailing slash
 */
export async function emitManifest({ outDir, siteDir, docsDir, versionedDocsDir, baseUrl }) {
  // --- collect current docs ---
  const currentRoot = path.join(siteDir, docsDir);
  const currentFiles = await walkDocs(currentRoot);
  const currentEntries = await Promise.all(
    currentFiles.map((f) => parseDoc(f, currentRoot, baseUrl, "current"))
  );

  // --- collect versioned docs ---
  const versionedRoot = path.join(siteDir, versionedDocsDir);
  let versionedEntries = [];
  let versionDirs;
  try {
    versionDirs = await fs.readdir(versionedRoot, { withFileTypes: true });
  } catch {
    versionDirs = [];
  }
  for (const entry of versionDirs) {
    if (!entry.isDirectory()) continue;
    const match = entry.name.match(/^version-(.+)$/);
    if (!match) continue;
    const versionLabel = match[1];
    const vRoot = path.join(versionedRoot, entry.name);
    const vFiles = await walkDocs(vRoot);
    const parsed = await Promise.all(
      vFiles.map((f) => parseDoc(f, vRoot, baseUrl, versionLabel))
    );
    versionedEntries.push(...parsed);
  }

  const allEntries = [...currentEntries, ...versionedEntries];

  // --- write llms.txt (all versions, tab-separated) ---
  const llmsTxtLines = allEntries.map(
    ({ slug, title, section, url, version }) =>
      `${slug}\t${title}\t${section}\t${url}\t${version}`
  );
  await fs.writeFile(path.join(outDir, "llms.txt"), llmsTxtLines.join("\n") + "\n", "utf8");

  // --- write llms-full.txt (current version only) ---
  const fullParts = currentEntries.map(
    ({ slug, title, url, body }) =>
      `## ${slug || "index"}\n> title: ${title}\n> version: current\n> url: ${url}\n\n${body}\n`
  );
  const fullContent = fullParts.join("\n---\n\n");

  const fullBytes = Buffer.byteLength(fullContent, "utf8");
  if (fullBytes > MAX_FULL_SIZE) {
    throw new Error(
      `llms-full.txt exceeds 2 MiB (${fullBytes} bytes); split content or increase budget`
    );
  }

  await fs.writeFile(path.join(outDir, "llms-full.txt"), fullContent, "utf8");
}
