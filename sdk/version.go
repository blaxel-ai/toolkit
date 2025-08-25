package sdk

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	version     string
	versionOnce sync.Once
	// commitHash can be set at build time via ldflags: -ldflags "-X github.com/blaxel-ai/toolkit/sdk.commitHash=abc1234"
	commitHash string
)

// GetVersion returns the SDK version automatically detected from module info
func GetVersion() string {
	versionOnce.Do(func() {
		version = detectVersion()
	})
	return version
}

// detectVersion attempts to get the module version from build info
func detectVersion() string {
	// Try to get version from build info (works when used as a module dependency)
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		// Look for our module in the dependencies
		for _, dep := range buildInfo.Deps {
			if strings.Contains(dep.Path, "blaxel-ai/toolkit") {
				if dep.Version != "" && dep.Version != "(devel)" {
					// Clean up version string (remove 'v' prefix if present)
					return strings.TrimPrefix(dep.Version, "v")
				}
			}
		}

		// If this is the main module and has version info
		if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			return strings.TrimPrefix(buildInfo.Main.Version, "v")
		}
	}

	// Fallback to "dev" if we can't determine the version
	return "dev"
}

// GetOsArch returns the operating system and architecture
func GetOsArch() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// GetCommitHash returns the commit hash from build-time injection
func GetCommitHash() string {
	// Check if commit hash was injected at build time via ldflags
	if commitHash != "" {
		if len(commitHash) > 7 {
			return commitHash[:7]
		}
		return commitHash
	}

	return "unknown"
}
