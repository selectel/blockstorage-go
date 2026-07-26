package blockstorage

import (
	"runtime/debug"
	"strings"
)

const (
	// appName represents an application name that is reported in the User-Agent.
	appName = "blockstorage-go"

	// modulePath represents the canonical public module path of the SDK.
	modulePath = "github.com/selectel/" + appName

	// unknownModuleVersion is reported when the build info does not contain the SDK version.
	unknownModuleVersion = "v0.0.0"
)

// buildUserAgent returns the User-Agent of the SDK, optionally prefixed by a caller-provided value.
func buildUserAgent(prefix string) string {
	userAgent := appName + "/" + moduleVersion()

	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return userAgent
	}

	return prefix + " " + userAgent
}

// moduleVersion returns the version of the SDK module taken from the build info of the caller.
func moduleVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownModuleVersion
	}

	for _, dependency := range buildInfo.Deps {
		if dependency.Path == modulePath {
			return dependency.Version
		}
	}

	return unknownModuleVersion
}
