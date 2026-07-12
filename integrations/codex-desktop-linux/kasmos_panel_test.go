package codexdesktoplinux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/internal/appwidget"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKasmosPanelContract(t *testing.T) {
	root := repositoryRoot(t)
	featureDir := filepath.Join(root, "integrations", "codex-desktop-linux", "kasmos-panel")
	hostSource := readFile(t, filepath.Join(root, "web", "admin", "src", "widget", "host.ts"))
	adapterSource := readFile(t, filepath.Join(featureDir, "host.js"))

	var feature struct {
		DefaultEnabled  bool `json:"defaultEnabled"`
		ContractVersion int  `json:"contract_version"`
	}
	require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(featureDir, "feature.json"))), &feature))
	assert.False(t, feature.DefaultEnabled)
	assert.Equal(t, appwidget.MonitorContractVersion, feature.ContractVersion)
	assert.Equal(t, appwidget.MonitorContractVersion, extractIntegerConstant(t, hostSource, "MONITOR_CONTRACT_VERSION"))
	assert.Equal(t, livestatus.SchemaVersion, extractIntegerConstant(t, hostSource, "LIVE_STATUS_SCHEMA_VERSION"))

	manifest, err := appwidget.Export(t.TempDir())
	require.NoError(t, err)
	required := extractStringArray(t, hostSource, "REQUIRED_HOST_CAPABILITIES")
	optional := extractStringArray(t, hostSource, "OPTIONAL_HOST_CAPABILITIES")
	assert.ElementsMatch(t, manifest.RequiredCapabilities, required)
	assert.ElementsMatch(t, manifest.OptionalCapabilities, optional)
	for _, capability := range required {
		assert.Regexpf(t, regexp.MustCompile(`(?m)(?:\b|get\s+)`+regexp.QuoteMeta(capability)+`\s*(?::|\(|\{)`), adapterSource, "host.js must implement %s", capability)
	}

	assert.NotContains(t, adapterSource, "fetch(")
	assert.NotContains(t, adapterSource, "task_transition")
	assert.NotContains(t, adapterSource, "implement_start")
	assert.NotContains(t, adapterSource, "instance_")
	for _, verb := range []string{"merge", "push", "delete"} {
		assert.NotRegexp(t, regexp.MustCompile(`(?i)\b`+verb+`\b`), adapterSource)
	}
	toolNames := regexp.MustCompile(`["']([a-z_]+monitor)["']`).FindAllStringSubmatch(adapterSource, -1)
	for _, match := range toolNames {
		assert.Equal(t, "refresh_monitor", match[1])
	}
	assert.Contains(t, adapterSource, "manifest.snapshot_endpoint.path")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find repository root from working directory")
		dir = parent
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func extractIntegerConstant(t *testing.T, source, name string) int {
	t.Helper()
	match := regexp.MustCompile(`(?m)export const ` + regexp.QuoteMeta(name) + `\s*=\s*(\d+)`).FindStringSubmatch(source)
	require.Len(t, match, 2, "%s must declare an integer literal", name)
	value, err := strconv.Atoi(match[1])
	require.NoError(t, err)
	return value
}

func extractStringArray(t *testing.T, source, name string) []string {
	t.Helper()
	match := regexp.MustCompile(`(?s)export const ` + regexp.QuoteMeta(name) + `\s*=\s*\[(.*?)\]\s*as const`).FindStringSubmatch(source)
	require.Len(t, match, 2, "%s must declare an array literal", name)
	quoted := regexp.MustCompile(`["']([^"']+)["']`).FindAllStringSubmatch(match[1], -1)
	values := make([]string, 0, len(quoted))
	for _, item := range quoted {
		values = append(values, strings.TrimSpace(item[1]))
	}
	sort.Strings(values)
	return values
}
