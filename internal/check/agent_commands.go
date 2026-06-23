package check

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kastheco/kasmos/config"
)

var resolveAgentCommandPath = config.ResolveCommandPath

// SetResolveAgentCommandPathForTest swaps the command resolver seam for tests.
func SetResolveAgentCommandPathForTest(fn func(string) (string, error)) func(string) (string, error) {
	prev := resolveAgentCommandPath
	resolveAgentCommandPath = fn
	return prev
}

// AgentCommandStatus is one [agents.*].program executable validation result.
type AgentCommandStatus struct {
	Role     string
	Program  string
	Resolved string
	Healthy  bool
	Detail   string
}

// AgentCommandResult reports executable health for configured agent profiles.
type AgentCommandResult struct {
	ConfigPath string
	LoadError  string
	Entries    []AgentCommandStatus
}

// AuditAgentCommands validates enabled [agents.*].program entries in config.toml.
func AuditAgentCommands(projectDir string) *AgentCommandResult {
	configPath := filepath.Join(projectDir, ".kasmos", config.TOMLConfigFileName)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return &AgentCommandResult{ConfigPath: configPath, LoadError: fmt.Sprintf("stat config: %v", err)}
	}

	tomlCfg, err := config.LoadTOMLConfigFrom(configPath)
	if err != nil {
		return &AgentCommandResult{ConfigPath: configPath, LoadError: err.Error()}
	}

	roles := make([]string, 0, len(tomlCfg.Profiles))
	for role := range tomlCfg.Profiles {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	result := &AgentCommandResult{ConfigPath: configPath}
	for _, role := range roles {
		profile := tomlCfg.Profiles[role]
		if !profile.Enabled {
			continue
		}
		status := AgentCommandStatus{
			Role:    role,
			Program: profile.Program,
		}
		status.Resolved, status.Detail, status.Healthy = validateAgentCommand(profile.Program, projectDir)
		result.Entries = append(result.Entries, status)
	}
	return result
}

func validateAgentCommand(program, projectDir string) (resolved, detail string, healthy bool) {
	token := commandExecutableToken(program)
	if token == "" {
		return "", "empty program", false
	}

	if strings.Contains(token, string(os.PathSeparator)) {
		path := token
		if !filepath.IsAbs(path) {
			path = filepath.Join(projectDir, path)
		}
		if err := executableFile(path); err != nil {
			return path, err.Error(), false
		}
		return path, "", true
	}

	path, err := resolveAgentCommandPath(token)
	if err != nil || path == "" {
		return "", fmt.Sprintf("%s not found in shell aliases or PATH", token), false
	}
	if strings.Contains(path, string(os.PathSeparator)) {
		if err := executableFile(path); err != nil {
			return path, err.Error(), false
		}
		return path, "", true
	}

	lookedUp, lookErr := exec.LookPath(path)
	if lookErr != nil {
		return path, lookErr.Error(), false
	}
	return lookedUp, "", true
}

func commandExecutableToken(program string) string {
	fields := strings.Fields(program)
	for _, field := range fields {
		if strings.Contains(field, "=") && !strings.Contains(field, string(os.PathSeparator)) {
			continue
		}
		return field
	}
	return ""
}

func executableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("is a directory")
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf("not executable")
	}
	return nil
}
