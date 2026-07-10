package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	})

	t.Run("MCP endpoint matches canonical URL", func(t *testing.T) {
		var config struct {
			MCPServers map[string]struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"mcpServers"`
		}
		readJSON(t, ".mcp.json", &config)

		require.Len(t, config.MCPServers, 1)
		server, ok := config.MCPServers["kasmos"]
		require.True(t, ok, "mcpServers must contain kasmos")
		assert.Equal(t, "http", server.Type)
		assert.Equal(t, mcpclient.SharedEndpointURL, server.URL)
	})

	t.Run("coordination skill is valid and synchronized", func(t *testing.T) {
		var manifest struct {
			Skills string `json:"skills"`
		}
		readJSON(t, ".codex-plugin/plugin.json", &manifest)
		skillPath := filepath.Join(manifest.Skills, "coordinate-kasmos", "SKILL.md")
		codexSkill, err := os.ReadFile(skillPath)
		require.NoError(t, err)

		frontmatter := parseFrontmatter(t, codexSkill)
		assert.Equal(t, "coordinate-kasmos", frontmatter.Name)
		assert.NotEmpty(t, frontmatter.Description)

		openclawSkill, err := os.ReadFile("../openclaw/skills/coordinate-kasmos/SKILL.md")
		require.NoError(t, err)
		assert.True(t, bytes.Equal(codexSkill, openclawSkill), "Codex and OpenClaw coordination skills must be byte-identical")
	})

	t.Run("marketplace names the plugin and its source", func(t *testing.T) {
		var marketplace struct {
			Plugins []struct {
				Name   string `json:"name"`
				Source struct {
					Source string `json:"source"`
				} `json:"source"`
			} `json:"plugins"`
		}
		readJSON(t, "marketplace.json", &marketplace)

		require.Len(t, marketplace.Plugins, 1)
		assert.Equal(t, "kasmos", marketplace.Plugins[0].Name)
		assert.NotEmpty(t, marketplace.Plugins[0].Source.Source)
	})
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
