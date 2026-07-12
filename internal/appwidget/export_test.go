package appwidget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportIntegritySafetyAndIdempotence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bundle")
	manifest, err := Export(dir)
	require.NoError(t, err)
	before := map[string][]byte{}
	for _, file := range manifest.Files {
		data, readErr := os.ReadFile(filepath.Join(dir, file.Path))
		require.NoError(t, readErr)
		sum := sha256.Sum256(data)
		assert.Equal(t, len(data), file.Bytes)
		assert.Equal(t, hex.EncodeToString(sum[:]), file.SHA256)
		before[file.Path] = data
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, manifestFilename))
	require.NoError(t, err)
	before[manifestFilename] = manifestData
	var emitted Manifest
	require.NoError(t, json.Unmarshal(manifestData, &emitted))
	assert.Equal(t, MonitorContractVersion, emitted.ContractVersion)
	assert.Equal(t, livestatus.SchemaVersion, emitted.LiveStatusSchemaVersion)
	assert.Equal(t, SnapshotPath, emitted.SnapshotEndpoint.Path)

	_, err = Export(dir)
	require.NoError(t, err)
	for name, want := range before {
		got, readErr := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, readErr)
		assert.Equal(t, want, got)
	}

	foreign := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("keep"), 0o644))
	_, err = Export(foreign)
	require.Error(t, err)
	entries, err := os.ReadDir(foreign)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "keep.txt", entries[0].Name())
}

func TestExportIndexHTMLIsSelfContained(t *testing.T) {
	dir := t.TempDir()
	_, err := Export(dir)
	require.NoError(t, err)
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	require.NoError(t, err)
	for _, forbidden := range []string{" src=", " href=", "@import", "//cdn", `src="https://`, `href="https://`, "web font", "remote image"} {
		assert.NotContains(t, string(html), forbidden)
	}
}
