package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

func Run() error {
	fmt.Println("kasmos setup")
	fmt.Println()

	fmt.Println("Checking dependencies...")
	results := CheckDependencies()
	allFound := true
	for _, r := range results {
		if r.Found {
			fmt.Printf("  [OK]      %-12s found (%s)\n", r.Name, r.Path)
			continue
		}

		fmt.Printf("  [MISSING] %-12s NOT FOUND\n", r.Name)
		if r.InstallHint != "" {
			fmt.Printf("    Install: %s\n", r.InstallHint)
		}
		if r.Required {
			allFound = false
		}
	}
	fmt.Println()

	if !allFound {
		return fmt.Errorf("required dependencies missing")
	}

	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	fmt.Println("Scaffolding agent definitions...")
	created, skipped, err := WriteAgentDefinitions(root)
	if err != nil {
		return fmt.Errorf("write agents: %w", err)
	}
	fmt.Printf("  %d created, %d skipped (already exist)\n", created, skipped)
	fmt.Println()
	fmt.Println("Setup complete!")

	return nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod or .git found in parent directories")
		}
		dir = parent
	}
}
