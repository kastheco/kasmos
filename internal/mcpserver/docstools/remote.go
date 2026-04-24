package docstools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// remoteCacheKey returns the cache map key for the given version and whether we
// are fetching the full manifest (llms-full.txt vs llms.txt).
func (d *Dispatcher) remoteCacheKey(version string, full bool) string {
	suffix := "llms.txt"
	if full {
		suffix = "llms-full.txt"
	}
	if version == "" {
		return d.baseURL + suffix
	}
	return d.baseURL + version + "/" + suffix
}

// manifestURL returns the llms-full.txt URL for the given version.
func (d *Dispatcher) manifestURL(version string) string {
	base := strings.TrimSuffix(d.baseURL, "/") + "/"
	if version == "" {
		return base + "llms-full.txt"
	}
	return base + version + "/llms-full.txt"
}

// fetchManifest fetches the llms-full.txt manifest, using in-process ETag caching
// to avoid redundant transfers. The host in the constructed URL must match
// d.allowedHost; requests to other hosts are rejected.
func (d *Dispatcher) fetchManifest(ctx context.Context, version string) (string, error) {
	if d.baseURL == "" {
		return "", fmt.Errorf("docs remote: remote mode is disabled (KASMOS_DOCS_BASE_URL is empty)")
	}
	manifestURL := d.manifestURL(version)

	// Verify the URL host is on the allowlist.
	u, err := url.Parse(manifestURL)
	if err != nil {
		return "", fmt.Errorf("docs remote: parse URL: %w", err)
	}
	if u.Host != d.allowedHost {
		return "", fmt.Errorf("docs remote: host %q not allowed (expected %q)", u.Host, d.allowedHost)
	}

	cacheKey := d.remoteCacheKey(version, true)

	d.cache.mu.Lock()
	existing, hasCached := d.cache.entries[cacheKey]
	d.cache.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("docs remote: build request: %w", err)
	}
	if hasCached && existing.etag != "" {
		req.Header.Set("If-None-Match", existing.etag)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("docs remote: fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// Cache hit: return the previously stored body. If the cache entry is
		// missing (e.g. eviction or unexpected server 304), return an error
		// rather than silently returning an empty manifest.
		d.cache.mu.Lock()
		entry, ok := d.cache.entries[cacheKey]
		d.cache.mu.Unlock()
		if !ok || entry.body == "" {
			return "", fmt.Errorf("docs remote: 304 received but no cached manifest for %s", manifestURL)
		}
		return entry.body, nil

	case http.StatusOK:
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("docs remote: read body: %w", readErr)
		}
		body := string(bodyBytes)
		d.cache.mu.Lock()
		d.cache.entries[cacheKey] = remoteEntry{
			body: body,
			etag: resp.Header.Get("ETag"),
		}
		d.cache.mu.Unlock()
		return body, nil

	default:
		return "", fmt.Errorf("docs remote: unexpected status %d from %s", resp.StatusCode, manifestURL)
	}
}

// searchRemote fetches the remote manifest and performs a substring search,
// scoring sections by occurrence count and returning the top N matches.
func (d *Dispatcher) searchRemote(ctx context.Context, pattern, version string, limit int) (*DocSearchResult, error) {
	body, err := d.fetchManifest(ctx, version)
	if err != nil {
		return nil, err
	}
	matches := d.searchManifest(body, pattern, version, limit)
	return &DocSearchResult{
		Matches: matches,
		Total:   len(matches),
		Source:  "remote",
	}, nil
}

// searchManifest splits body by "## <slug>" headers, scores by occurrence count,
// and returns the top N matches as DocMatch values.
func (d *Dispatcher) searchManifest(body, pattern, version string, limit int) []DocMatch {
	// Normalise: if the body starts with "## " (no preamble) strip it so the
	// subsequent split on "\n## " handles all sections uniformly.
	body = strings.TrimPrefix(body, "## ")
	sections := strings.Split(body, "\n## ")

	type scored struct {
		slug    string
		content string
		count   int
	}
	candidates := make([]scored, 0, len(sections))
	lowerPat := strings.ToLower(pattern)

	for _, section := range sections {
		if section == "" {
			continue
		}
		// First line of the section is the slug.
		newline := strings.Index(section, "\n")
		var slug, content string
		if newline < 0 {
			slug = strings.TrimSpace(section)
		} else {
			slug = strings.TrimSpace(section[:newline])
			content = section[newline+1:]
		}
		if slug == "" {
			continue
		}
		count := strings.Count(strings.ToLower(slug+"\n"+content), lowerPat)
		if count == 0 {
			continue
		}
		candidates = append(candidates, scored{slug: slug, content: content, count: count})
	}

	// Sort descending by occurrence count (insertion sort; N is typically small).
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].count > candidates[j-1].count; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	matches := make([]DocMatch, 0, min(len(candidates), limit))
	for i, c := range candidates {
		if i >= limit {
			break
		}
		snippet := strings.TrimSpace(c.content)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		matches = append(matches, DocMatch{
			Slug:    c.slug,
			URL:     d.slugToURL(c.slug, version),
			Snippet: snippet,
			Source:  "remote",
		})
	}
	return matches
}

// readRemote fetches the remote manifest and extracts a single section by slug.
func (d *Dispatcher) readRemote(ctx context.Context, slug, version string) (*DocReadResult, error) {
	body, err := d.fetchManifest(ctx, version)
	if err != nil {
		return nil, err
	}

	body = strings.TrimPrefix(body, "## ")
	sections := strings.Split(body, "\n## ")
	for _, section := range sections {
		newline := strings.Index(section, "\n")
		if newline < 0 {
			continue
		}
		sectionSlug := strings.TrimSpace(section[:newline])
		if sectionSlug != slug {
			continue
		}
		content := section[newline+1:]
		ver := version
		if ver == "" {
			ver = "current"
		}
		return &DocReadResult{
			Slug:    slug,
			Version: ver,
			URL:     d.slugToURL(slug, version),
			Content: content,
			Source:  "remote",
		}, nil
	}
	return nil, fmt.Errorf("docs_read: slug %q not found in remote manifest", slug)
}
