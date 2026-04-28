package daemon

import (
	"path/filepath"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/session"
)

func resolvedNamedProfile(repoPath, profileName string) (config.AgentProfile, bool) {
	configPath := filepath.Join(repoPath, ".kasmos", config.TOMLConfigFileName)
	result, err := config.LoadTOMLConfigFrom(configPath)
	if err != nil {
		return config.AgentProfile{}, false
	}
	cfg := &config.Config{
		Profiles: result.Profiles,
	}
	profile, ok := cfg.ResolveNamedProfile(profileName, result.DefaultProgram)
	return profile, ok
}

func programForNamedProfile(repoPath, profileName string) string {
	return programForNamedProfileWithRegistry(repoPath, profileName, harness.NewRegistry())
}

func programForNamedProfileWithRegistry(repoPath, profileName string, registry *harness.Registry) string {
	profile, ok := resolvedNamedProfile(repoPath, profileName)
	if !ok {
		return ""
	}
	return buildProgramCommand(profile, registry)
}

func executionModeForNamedProfile(repoPath, profileName string) session.ExecutionMode {
	profile, ok := resolvedNamedProfile(repoPath, profileName)
	if !ok {
		return ""
	}
	return session.ExecutionMode(config.NormalizeExecutionMode(profile.ExecutionMode))
}

func sdkSpeedTierForNamedProfile(repoPath, profileName string) string {
	profile, ok := resolvedNamedProfile(repoPath, profileName)
	if !ok {
		return ""
	}
	return session.NormalizeSDKSpeedTier(profile.Tier)
}

func skipPermissionsForNamedProfile(repoPath, profileName string) bool {
	profile, ok := resolvedNamedProfile(repoPath, profileName)
	if !ok {
		return true
	}
	return profile.ResolveSkipPermissions(true)
}
