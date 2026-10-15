// Package version holds build-time version metadata injected via -ldflags.
package version

import "strings"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Tag is Version as a release tag, "v1.2.3". GoReleaser sets Version without the "v".
func Tag() string {
	if Version == "dev" || strings.HasPrefix(Version, "v") {
		return Version
	}
	return "v" + Version
}
