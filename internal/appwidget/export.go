package appwidget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	webassets "github.com/kastheco/kasmos/web"
)

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type SnapshotEndpoint struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	DefaultOrigin string `json:"default_origin"`
}

type Manifest struct {
	Name                    string           `json:"name"`
	ContractVersion         int              `json:"contract_version"`
	LiveStatusSchemaVersion int              `json:"live_status_schema_version"`
	RootElementID           string           `json:"root_element_id"`
	HostGlobal              string           `json:"host_global"`
	SnapshotEndpoint        SnapshotEndpoint `json:"snapshot_endpoint"`
	RequiredCapabilities    []string         `json:"required_capabilities"`
	OptionalCapabilities    []string         `json:"optional_capabilities"`
	Files                   []ManifestFile   `json:"files"`
}

const manifestFilename = "kasmos-monitor.manifest.json"

// Export writes the monitor bundle and its integrity manifest into dir.
func Export(dir string) (Manifest, error) {
	owned := map[string]bool{"monitor.js": true, "monitor.css": true, "index.html": true, manifestFilename: true}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !owned[entry.Name()] {
				return Manifest{}, fmt.Errorf("refusing to export into directory containing foreign entry %q", entry.Name())
			}
		}
	} else if !os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("inspect output directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create output directory: %w", err)
	}
	js, css := webassets.MonitorBundle()
	files := []struct {
		name string
		data []byte
	}{
		{"monitor.js", []byte(js)}, {"monitor.css", []byte(css)}, {"index.html", []byte(WidgetHTML())},
	}
	manifestFiles := make([]ManifestFile, 0, len(files))
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.data, 0o644); err != nil {
			return Manifest{}, fmt.Errorf("write %s: %w", file.name, err)
		}
		sum := sha256.Sum256(file.data)
		manifestFiles = append(manifestFiles, ManifestFile{Path: file.name, SHA256: hex.EncodeToString(sum[:]), Bytes: len(file.data)})
	}
	endpoint, err := url.Parse(DefaultSnapshotEndpoint)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse default snapshot endpoint: %w", err)
	}
	manifest := Manifest{Name: "kasmos-monitor", ContractVersion: MonitorContractVersion, LiveStatusSchemaVersion: LiveStatusSchemaVersion, RootElementID: "root", HostGlobal: "kasmosMonitorHost", SnapshotEndpoint: SnapshotEndpoint{Method: "POST", Path: SnapshotPath, DefaultOrigin: endpoint.Scheme + "://" + endpoint.Host}, RequiredCapabilities: []string{"contractVersion", "displayMode", "visibility", "theme", "refresh", "subscribe"}, OptionalCapabilities: []string{"saveState", "setBadge", "requestDisplayMode", "sendPrompt"}, Files: manifestFiles}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode manifest: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(dir, manifestFilename), encoded, 0o644); err != nil {
		return Manifest{}, fmt.Errorf("write manifest: %w", err)
	}
	return manifest, nil
}
