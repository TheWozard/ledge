package version

import (
	"runtime/debug"
	"testing"
)

func TestFromBuildInfo(t *testing.T) {
	defer func(v, c, d string) { Version, Commit, Date = v, c, d }(Version, Commit, Date)
	for _, c := range []struct {
		version, want string
	}{
		{"v1.2.3", "v1.2.3"},
		{"v0.0.0-20261026120000-abcdef123456", "dev"},
		{"v1.2.4-0.20261026120000-abcdef123456+dirty", "dev"},
		{"(devel)", "dev"},
	} {
		Version, Commit, Date = "dev", "none", "unknown"
		fromBuildInfo(&debug.BuildInfo{Main: debug.Module{Version: c.version}, Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"}, {Key: "vcs.time", Value: "2026-10-26T12:00:00Z"},
		}})
		if Version != c.want || Commit != "abcdef1" || Date != "2026-10-26T12:00:00Z" {
			t.Errorf("%s: %s (%s, %s); want %s (abcdef1, 2026-10-26T12:00:00Z)", c.version, Version, Commit, Date, c.want)
		}
	}
	fromBuildInfo(&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abcdef1234567890"}, {Key: "vcs.modified", Value: "true"}}})
	if Commit != "abcdef1+dirty" {
		t.Errorf("a modified checkout's commit = %s", Commit)
	}
}
