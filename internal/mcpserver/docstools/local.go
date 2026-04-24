package docstools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// rgLine is the top-level structure of a single rg --json output line.
type rgLine struct {
	Type string `json:"type"`
	Data rgData `json:"data"`
}

type rgData struct {
	Path       rgText `json:"path"`
	Lines      rgText `json:"lines"`
	LineNumber int    `json:"line_number"`
}

type rgText struct {
	Text string `json:"text"`
}

// searchLocal runs rg --json against the given docs root and returns matches.
func (d *Dispatcher) searchLocal(ctx context.Context, root, pattern, version, pathGlob string, limit, contextLines int) (*DocSearchResult, error) {
	args := []string{"--json", "--no-messages", "-g", "*.mdx", "-g", "*.md"}
	if pathGlob != "" {
		args = append(args, "-g", pathGlob)
	}
	if contextLines > 0 {
		args = append(args, "-C", strconv.Itoa(contextLines))
	}
	args = append(args, pattern, root)

	out, err := d.runner.Output(ctx, "rg", args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 1:
				// Exit code 1: rg found no matches — not an error.
				return &DocSearchResult{Matches: []DocMatch{}, Source: "local"}, nil
			case 2:
				// Exit code 2: rg encountered errors (e.g. permission denied on some
				// directories) but may have produced partial results on stdout.
				if len(out) > 0 {
					if partial, parseErr := parseDocsRgJSON(out, root, version, d.baseURL, limit); parseErr == nil && len(partial) > 0 {
						return &DocSearchResult{Matches: partial, Total: len(partial), Source: "local"}, nil
					}
				}
				return &DocSearchResult{Matches: []DocMatch{}, Source: "local"}, nil
			}
		}
		return nil, fmt.Errorf("docs_search: rg failed: %w", err)
	}

	matches, err := parseDocsRgJSON(out, root, version, d.baseURL, limit)
	if err != nil {
		return nil, fmt.Errorf("docs_search: parse output: %w", err)
	}
	return &DocSearchResult{Matches: matches, Total: len(matches), Source: "local"}, nil
}

// parseDocsRgJSON parses the NDJSON output of `rg --json` into a slice of
// DocMatch, converting file paths to slugs and constructing URLs.
func parseDocsRgJSON(data []byte, root, version, baseURL string, limit int) ([]DocMatch, error) {
	matches := make([]DocMatch, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Increase buffer for long rg JSON lines (mirrors fstools/grep.go).
	scanner.Buffer(make([]byte, 0, 256*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rl rgLine
		if err := json.Unmarshal(line, &rl); err != nil {
			return nil, fmt.Errorf("parse rg JSON: %w", err)
		}
		if rl.Type != "match" {
			continue
		}
		if len(matches) >= limit {
			break
		}
		slug := filePathToSlug(rl.Data.Path.Text, root)
		docURL := slugToURLWithBase(slug, version, baseURL)
		matches = append(matches, DocMatch{
			Slug:    slug,
			URL:     docURL,
			Path:    rl.Data.Path.Text,
			Line:    rl.Data.LineNumber,
			Snippet: strings.TrimRight(rl.Data.Lines.Text, "\n"),
			Source:  "local",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan rg output: %w", err)
	}
	return matches, nil
}

// filePathToSlug converts an absolute file path to a doc slug by stripping the
// docs-root prefix and file extension.
func filePathToSlug(filePath, root string) string {
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		rel = filePath
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".mdx")
	rel = strings.TrimSuffix(rel, ".md")
	// Strip a trailing /index produced by index.mdx files.
	rel = strings.TrimSuffix(rel, "/index")
	return rel
}

// readLocal reads a single doc file from the local docs tree. It resolves the
// slug to <root>/<slug>.mdx, <root>/<slug>.md, or <root>/<slug>/index.mdx, and
// parses any YAML frontmatter.
func (d *Dispatcher) readLocal(_ context.Context, root, slug, version string) (*DocReadResult, error) {
	cleanRoot := filepath.Clean(root)
	candidates := []string{
		filepath.Join(root, filepath.FromSlash(slug)+".mdx"),
		filepath.Join(root, filepath.FromSlash(slug)+".md"),
		filepath.Join(root, filepath.FromSlash(slug), "index.mdx"),
		filepath.Join(root, filepath.FromSlash(slug), "index.md"),
	}

	var filePath string
	for _, c := range candidates {
		// Guard against path traversal via slug.
		if !strings.HasPrefix(filepath.Clean(c), cleanRoot+string(os.PathSeparator)) {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			filePath = c
			break
		}
	}
	if filePath == "" {
		return nil, fmt.Errorf("docs_read: %q not found locally", slug)
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("docs_read: read %q: %w", filePath, err)
	}

	fm, content := parseFrontmatter(string(raw))
	title := fm["title"]
	ver := version
	if ver == "" {
		ver = "current"
	}

	return &DocReadResult{
		Slug:        slug,
		Title:       title,
		Version:     ver,
		URL:         slugToURLWithBase(slug, version, d.baseURL),
		Content:     content,
		Frontmatter: fm,
		Source:      "local",
	}, nil
}

// parseFrontmatter splits YAML frontmatter from MDX/MD content. It handles the
// simple string-only frontmatter format used across web/docs/docs/**/*.mdx.
// Returns (frontmatter map, body content without the frontmatter block).
func parseFrontmatter(raw string) (map[string]string, string) {
	fm := make(map[string]string)
	if !strings.HasPrefix(raw, "---\n") {
		return fm, raw
	}
	// Find the closing "---" line.
	rest := raw[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return fm, raw
	}
	fmBlock := rest[:idx]
	content := rest[idx+5:] // skip "\n---\n"

	for _, line := range strings.Split(fmBlock, "\n") {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		// Strip surrounding single or double quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			fm[key] = val
		}
	}
	return fm, content
}
