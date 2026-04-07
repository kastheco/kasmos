package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webassets "github.com/kastheco/kasmos/web"
)

func TestAdminFallbackHandler_ServesIndex(t *testing.T) {
	handler := adminFallbackHandler(http.Dir("testdata/admin"))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Root serves index.html
	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.True(t, strings.Contains(string(body), "kas admin"))
}

func TestAdminFallbackHandler_SPAFallback(t *testing.T) {
	handler := adminFallbackHandler(http.Dir("testdata/admin"))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Non-existent path without extension falls back to index.html (SPA routing)
	resp, err := http.Get(srv.URL + "/tasks/some-plan")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.True(t, strings.Contains(string(body), "kas admin"))
}

func TestAdminFallbackHandler_DotInSPARoute(t *testing.T) {
	handler := adminFallbackHandler(http.Dir("testdata/admin"))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// SPA routes that contain a dot (e.g. a plan filename like plan-foo.md)
	// must also fall back to index.html, not return a hard 404.
	resp, err := http.Get(srv.URL + "/tasks/plan-foo.md")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.True(t, strings.Contains(string(body), "kas admin"))
}

func TestAdminFallbackHandler_MissingAsset404(t *testing.T) {
	handler := adminFallbackHandler(http.Dir("testdata/admin"))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Missing compiled asset under /assets/ returns 404, not index.html.
	// Vite always places hashed JS/CSS/images under /assets/, so a missing
	// file there is always a genuine error rather than a SPA route.
	resp, err := http.Get(srv.URL + "/assets/missing-abc123.js")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestAdminEmbeddedAssetIntegrity verifies that every asset referenced in the
// embedded index.html actually exists in the embedded filesystem.  This is a
// regression guard against the broken-dist state where index.html referenced a
// hashed JS bundle that was not present in web/admin/dist/assets/.
func TestAdminEmbeddedAssetIntegrity(t *testing.T) {
	fsys := webassets.AdminFS()

	// Open and read index.html from the embedded root.
	f, err := fsys.Open("index.html")
	require.NoError(t, err, "embedded index.html must be present")
	defer f.Close()

	body, err := io.ReadAll(f)
	require.NoError(t, err)

	// Extract all src="..." and href="..." values that reference /admin/assets/.
	re := regexp.MustCompile(`(?:src|href)="(/admin/assets/[^"]+)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	require.NotEmpty(t, matches, "index.html must contain at least one /admin/assets/ reference")

	for _, m := range matches {
		ref := m[1] // e.g. /admin/assets/index-XxleArwf.js
		// Strip the leading /admin/ prefix: the FS is rooted at dist/.
		path := strings.TrimPrefix(ref, "/admin/")
		asset, openErr := fsys.Open(path)
		assert.NoErrorf(t, openErr, "embedded asset %q (from index.html reference %q) must exist", path, ref)
		if asset != nil {
			asset.Close()
		}
	}
}
