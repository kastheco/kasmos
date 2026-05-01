package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotEnvValues holds parsed key/value pairs from a dotenv file.
type DotEnvValues map[string]string

// LoadProjectDotEnv loads <repo-root>/.env into the process environment.
// Existing environment variables win so shell/systemd configuration can
// override project-local defaults.
func LoadProjectDotEnv() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	root := cwd
	if repoRoot, repoErr := ResolveRepoRoot(cwd); repoErr == nil {
		root = repoRoot
	}
	return LoadDotEnv(filepath.Join(root, ".env"))
}

// LoadDotEnv loads path into the process environment without overwriting
// already-set variables. Missing files are a no-op.
func LoadDotEnv(path string) error {
	values, err := ReadDotEnv(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	return nil
}

// ReadDotEnv parses a dotenv file. It supports the common KEY=value shape,
// optional "export", single/double quotes, blank lines, and whole-line comments.
func ReadDotEnv(path string) (DotEnvValues, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := DotEnvValues{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		key, value, ok, err := parseDotEnvLine(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if ok {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return values, nil
}

func parseDotEnvLine(line string) (string, string, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false, fmt.Errorf("invalid dotenv line")
	}
	key := strings.TrimSpace(line[:eq])
	if !validDotEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid dotenv key %q", key)
	}
	value := strings.TrimSpace(line[eq+1:])
	value, err := cleanDotEnvValue(value)
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

func validDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func cleanDotEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	quote := value[0]
	if quote == '\'' || quote == '"' {
		end := -1
		escaped := false
		for i := 1; i < len(value); i++ {
			if quote == '"' && value[i] == '\\' && !escaped {
				escaped = true
				continue
			}
			if value[i] == quote && !escaped {
				end = i
				break
			}
			escaped = false
		}
		if end < 0 {
			return "", fmt.Errorf("unterminated quoted dotenv value")
		}
		tail := strings.TrimSpace(value[end+1:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			return "", fmt.Errorf("invalid trailing content after quoted dotenv value")
		}
		value = value[1:end]
		if quote == '"' {
			value = strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t", `\"`, `"`, `\\`, `\`).Replace(value)
		}
		return value, nil
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value), nil
}
