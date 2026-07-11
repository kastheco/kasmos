package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/kastheco/kasmos/internal/mcpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPluginBundleContract(t *testing.T) {
	t.Run("manifest is structurally valid", func(t *testing.T) {
		var manifest struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Skills      string `json:"skills"`
			MCPServers  string `json:"mcpServers"`
			Interface   struct {
				Category      string   `json:"category"`
				DefaultPrompt []string `json:"defaultPrompt"`
			} `json:"interface"`
		}
		readJSON(t, ".codex-plugin/plugin.json", &manifest)

		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "name", value: manifest.Name},
			{name: "version", value: manifest.Version},
			{name: "description", value: manifest.Description},
		} {
			assert.NotEmpty(t, field.value, field.name)
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "skills", value: manifest.Skills},
			{name: "mcpServers", value: manifest.MCPServers},
		} {
			assert.Truef(t, strings.HasPrefix(field.value, "./"), "%s must begin with ./, got %q", field.name, field.value)
		}
		assert.Equal(t, "Developer Tools", manifest.Interface.Category)
		require.Len(t, manifest.Interface.DefaultPrompt, 1)
		assert.NotEmpty(t, manifest.Interface.DefaultPrompt[0])
	})

	t.Run("MCP endpoint matches canonical URL", func(t *testing.T) {
		var config map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}
		readJSON(t, ".mcp.json", &config)

		require.Len(t, config, 1)
		server, ok := config["kasmos"]
		require.True(t, ok, ".mcp.json must contain kasmos at the top level")
		assert.Equal(t, "http", server.Type)
		assert.Equal(t, mcpclient.SharedEndpointURL, server.URL)
	})

	bundledSkills := []string{"coordinate-kasmos", "monitor-kasmos-task"}

	t.Run("bundled skills are valid and synchronized", func(t *testing.T) {
		var manifest struct {
			Skills string `json:"skills"`
		}
		readJSON(t, ".codex-plugin/plugin.json", &manifest)

		for _, skill := range bundledSkills {
			t.Run(skill, func(t *testing.T) {
				for _, file := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml")} {
					codexBytes, err := os.ReadFile(filepath.Join(manifest.Skills, skill, file))
					require.NoError(t, err)
					openclawBytes, err := os.ReadFile(filepath.Join("..", "openclaw", "skills", skill, file))
					require.NoError(t, err)
					assert.Truef(t, bytes.Equal(codexBytes, openclawBytes),
						"Codex and OpenClaw copies of %s/%s must be byte-identical", skill, file)
				}

				skillBytes, err := os.ReadFile(filepath.Join(manifest.Skills, skill, "SKILL.md"))
				require.NoError(t, err)
				frontmatter := parseFrontmatter(t, skillBytes)
				assert.Equal(t, skill, frontmatter.Name)
				assert.NotEmpty(t, frontmatter.Description)
			})
		}
	})

	t.Run("monitor skill pins the live-status contract", func(t *testing.T) {
		skillBytes, err := os.ReadFile(filepath.Join("skills", "monitor-kasmos-task", "SKILL.md"))
		require.NoError(t, err)

		pin := monitorSkillPin(t, skillBytes)
		assert.Equal(t, livestatus.SchemaVersion, pin.LiveStatusSchemaVersion)
		assert.ElementsMatch(t, []string{"live_status", "task_list"}, pin.IdleTools)
		assert.Contains(t, string(skillBytes), "## Retirement")
		assert.Regexp(t, `(?s)## Retirement.*monitor`, string(skillBytes))
	})

	t.Run("marketplace names the plugin and its source", func(t *testing.T) {
		var marketplace struct {
			Plugins []struct {
				Name   string `json:"name"`
				Source struct {
					Source string `json:"source"`
					Path   string `json:"path"`
				} `json:"source"`
				Category string `json:"category"`
			} `json:"plugins"`
		}
		marketplacePath := filepath.Join(".agents", "plugins", "marketplace.json")
		readJSON(t, marketplacePath, &marketplace)

		require.Len(t, marketplace.Plugins, 1)
		plugin := marketplace.Plugins[0]
		assert.Equal(t, "kasmos", plugin.Name)
		assert.Equal(t, "local", plugin.Source.Source)
		assert.Equal(t, "Developer Tools", plugin.Category)

		marketplaceRoot := "."
		sourcePath := filepath.Clean(filepath.Join(marketplaceRoot, plugin.Source.Path))
		assert.Equal(t, ".", sourcePath)
		info, err := os.Stat(filepath.Join(sourcePath, ".codex-plugin", "plugin.json"))
		require.NoError(t, err)
		assert.False(t, info.IsDir())
	})
}

// monitorSkillPin extracts the first ```yaml fenced block from SKILL.md.
func monitorSkillPin(t *testing.T, skill []byte) struct {
	LiveStatusSchemaVersion int      `yaml:"live_status_schema_version"`
	IdleTools               []string `yaml:"idle_tools"`
	EscalationTools         []string `yaml:"escalation_tools"`
} {
	t.Helper()
	_, rest, found := strings.Cut(string(skill), "```yaml")
	require.True(t, found, "SKILL.md must contain a ```yaml contract-pin block")
	body, _, found := strings.Cut(rest, "```")
	require.True(t, found, "contract-pin block must be closed")

	var pin struct {
		LiveStatusSchemaVersion int      `yaml:"live_status_schema_version"`
		IdleTools               []string `yaml:"idle_tools"`
		EscalationTools         []string `yaml:"escalation_tools"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(body), &pin))
	return pin
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}

func parseFrontmatter(t *testing.T, skill []byte) struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
} {
	t.Helper()
	parts := bytes.SplitN(skill, []byte("---"), 3)
	require.Len(t, parts, 3, "SKILL.md must contain YAML frontmatter")
	require.Empty(t, bytes.TrimSpace(parts[0]), "frontmatter must start at the beginning of SKILL.md")

	var frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	require.NoError(t, yaml.Unmarshal(parts[1], &frontmatter))
	return frontmatter
}
