package wizard

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectTools(t *testing.T) {
	fakeLookup := func(binary string) (string, error) {
		switch binary {
		case "sg":
			return "/usr/bin/sg", nil
		case "sd":
			return "/usr/bin/sd", nil
		default:
			return "", &exec.Error{Name: binary, Err: exec.ErrNotFound}
		}
	}

	results := detectTools(toolCatalog, fakeLookup)

	t.Run("returns result for every catalog entry", func(t *testing.T) {
		assert.Len(t, results, len(toolCatalog))
	})

	t.Run("found tools have path and Found=true", func(t *testing.T) {
		for _, r := range results {
			if r.Binary == "sg" {
				assert.True(t, r.Found)
				assert.Equal(t, "/usr/bin/sg", r.Path)
			}
			if r.Binary == "sd" {
				assert.True(t, r.Found)
				assert.Equal(t, "/usr/bin/sd", r.Path)
			}
		}
	})

	t.Run("missing tools have Found=false", func(t *testing.T) {
		for _, r := range results {
			if r.Binary == "comby" {
				assert.False(t, r.Found)
				assert.Empty(t, r.Path)
			}
		}
	})
}
