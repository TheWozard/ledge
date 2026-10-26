// Package version holds the build's version metadata: set with -ldflags at release
// time, or else read from what Go records in the binary.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// release is a release's module version, "v1.2.3": not a pseudo-version such as
// "v0.0.0-20261026120000-abcdef123456", nor one with "+dirty".
var release = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	fromBuildInfo(info)
}

// fromBuildInfo fills in what -ldflags didn't set from info: the version, if it was
// installed as a release (`go install …@v1.2.3`), and the commit and its time, if it
// was built from a checkout: "+dirty" if the checkout had changes.
func fromBuildInfo(info *debug.BuildInfo) {
	if release.MatchString(info.Main.Version) {
		Version = info.Main.Version
	}
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			Commit = setting.Value[:min(len(setting.Value), 7)]
		case "vcs.time":
			Date = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if dirty {
		Commit += "+dirty"
	}
}

// Tag is Version as a release tag, "v1.2.3". GoReleaser sets Version without the "v".
func Tag() string {
	if Version == "dev" || strings.HasPrefix(Version, "v") {
		return Version
	}
	return "v" + Version
}
