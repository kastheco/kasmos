// Package docstools provides documentation search and read tool handlers for
// the kasmos MCP server. It exposes docs_search and docs_read tools that operate
// locally against web/docs/docs/** when the kasmos repo is checked out, and fall
// back to HTTPS fetches of llms-full.txt for downstream projects without the repo.
package docstools

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
)

// DocMatch mirrors fstools.GrepMatch but carries doc-semantic fields.
type DocMatch struct {
	Slug    string `json:"slug"`
	Title   string `json:"title,omitempty"`
	Section string `json:"section,omitempty"`
	URL     string `json:"url"`
	Path    string `json:"path,omitempty"` // repo-relative when local, empty when remote
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"` // "local" | "remote"
}

// DocSearchResult wraps search matches in an object for MCP structuredContent.
type DocSearchResult struct {
	Matches []DocMatch `json:"matches"`
	Total   int        `json:"total"`
	Source  string     `json:"source"` // "local" | "remote"
}

// DocReadResult holds a single documentation page's content and metadata.
type DocReadResult struct {
	Slug        string            `json:"slug"`
	Title       string            `json:"title,omitempty"`
	Version     string            `json:"version"` // "current" or "X.Y.Z"
	URL         string            `json:"url"`
	Content     string            `json:"content"`
	Frontmatter map[string]string `json:"frontmatter,omitempty"`
	Source      string            `json:"source"` // "local" | "remote"
}

// remoteEntry is a cached remote manifest body with its ETag.
type remoteEntry struct {
	body string
	etag string
}

// remoteCache holds in-process cached llms-full.txt content, keyed by URL.
type remoteCache struct {
	mu      sync.Mutex
	entries map[string]remoteEntry
}

// Dispatcher coordinates between local and remote documentation sources.
type Dispatcher struct {
	sb          *fstools.Sandbox
	allowedDirs []string
	runner      fstools.CmdRunner
	httpClient  *http.Client
	baseURL     string
	allowedHost string // parsed from baseURL at construction time
	cache       remoteCache
}

// NewDispatcher creates a Dispatcher. allowedDirs is used to locate the local
// docs root (web/docs/docs) under each configured directory.
func NewDispatcher(sb *fstools.Sandbox, allowedDirs []string, runner fstools.CmdRunner, httpClient *http.Client, baseURL string) *Dispatcher {
	allowedHost := ""
	if u, err := url.Parse(baseURL); err == nil {
		allowedHost = u.Host
	}
	return &Dispatcher{
		sb:          sb,
		allowedDirs: allowedDirs,
		runner:      runner,
		httpClient:  httpClient,
		baseURL:     baseURL,
		allowedHost: allowedHost,
		cache:       remoteCache{entries: make(map[string]remoteEntry)},
	}
}

// findDocsRoot returns the first matching local docs root for the given version,
// and whether it was found. When version is empty it looks for web/docs/docs;
// when version is non-empty it looks for web/docs/versioned_docs/version-<version>.
func (d *Dispatcher) findDocsRoot(version string) (string, bool) {
	for _, allowed := range d.allowedDirs {
		var root string
		if version == "" {
			root = filepath.Join(allowed, "web", "docs", "docs")
		} else {
			root = filepath.Join(allowed, "web", "docs", "versioned_docs", "version-"+version)
		}
		if _, err := os.Stat(root); err == nil {
			return root, true
		}
	}
	return "", false
}

// Search searches documentation. It uses the local rg-based path when a docs
// root is found under any allowed directory, and falls back to HTTPS otherwise.
func (d *Dispatcher) Search(ctx context.Context, pattern, version, pathGlob string, limit, contextLines int) (*DocSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	root, ok := d.findDocsRoot(version)
	if ok {
		return d.searchLocal(ctx, root, pattern, version, pathGlob, limit, contextLines)
	}
	return d.searchRemote(ctx, pattern, version, limit)
}

// Read reads a single doc page identified by target (slug, repo-relative path,
// or URL). It prefers the local docs tree, falling back to the remote manifest.
func (d *Dispatcher) Read(ctx context.Context, target, version string) (*DocReadResult, error) {
	slug, ver := d.normalizeTarget(target, version)
	root, ok := d.findDocsRoot(ver)
	if ok {
		return d.readLocal(ctx, root, slug, ver)
	}
	return d.readRemote(ctx, slug, ver)
}

// normalizeTarget converts target (slug / repo-relative path / URL) to
// (slug, version). If version is already set it is preserved.
func (d *Dispatcher) normalizeTarget(target, version string) (string, string) {
	// If target is a URL, extract the path component.
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if u, err := url.Parse(target); err == nil {
			target = u.Path
		}
	}
	// Strip base URL path prefix if present.
	if u, err := url.Parse(d.baseURL); err == nil {
		basePath := strings.TrimSuffix(u.Path, "/") + "/"
		if strings.HasPrefix(target, basePath) {
			target = strings.TrimPrefix(target, basePath)
		}
	}
	// Strip leading/trailing slashes and known extensions.
	target = strings.Trim(target, "/")
	target = strings.TrimSuffix(target, ".mdx")
	target = strings.TrimSuffix(target, ".md")
	// Strip repo-relative prefix for the current docs tree.
	if rest, ok := strings.CutPrefix(target, "web/docs/docs/"); ok {
		target = rest
	}
	// Strip repo-relative prefix for versioned docs tree.
	if rest, ok := strings.CutPrefix(target, "web/docs/versioned_docs/"); ok {
		// rest is "version-X.Y.Z/slug/..."
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "version-") {
			if version == "" {
				version = strings.TrimPrefix(parts[0], "version-")
			}
			target = parts[1]
		}
	}
	return target, version
}

// slugToURL converts a slug and optional version to a fully qualified docs URL.
func (d *Dispatcher) slugToURL(slug, version string) string {
	return slugToURLWithBase(slug, version, d.baseURL)
}

// slugToURLWithBase is the package-level helper used by both local and remote code.
func slugToURLWithBase(slug, version, baseURL string) string {
	base := strings.TrimSuffix(baseURL, "/") + "/"
	if version != "" {
		return base + version + "/" + slug + "/"
	}
	return base + slug + "/"
}
