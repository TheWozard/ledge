package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archive is a .tar.gz holding one file.
func archive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	files := tar.NewWriter(gz)
	if err := files.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	files.Write(content)
	files.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// fakeGitHub serves a latest release with the given files as its assets.
func fakeGitHub(t *testing.T, tag string, files map[string][]byte) *Client {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	release := Release{Tag: tag}
	for name, data := range files {
		release.Assets = append(release.Assets, Asset{Name: name, URL: server.URL + "/download/" + name})
		mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(data) })
	}
	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/releases/latest", Owner, Repo), func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	})
	return &Client{API: server.URL, HTTP: server.Client()}
}

func TestDownload(t *testing.T) {
	binary := []byte("#!/bin/sh\necho new ledge\n")
	data := archive(t, Binary, binary)
	name := AssetName()

	client := fakeGitHub(t, "v1.2.3", map[string][]byte{
		name:            data,
		"checksums.txt": []byte(sum(data) + "  " + name + "\n" + strings.Repeat("0", 64) + "  other.tar.gz\n"),
	})
	release, err := client.Latest()
	if err != nil || release.Tag != "v1.2.3" {
		t.Fatalf("Latest = %+v, %v", release, err)
	}
	got, err := client.Download(release)
	if err != nil || !bytes.Equal(got, binary) {
		t.Fatalf("Download = %q, %v", got, err)
	}
}

func TestDownloadRefuses(t *testing.T) {
	data := archive(t, Binary, []byte("ledge"))
	name := AssetName()
	for label, c := range map[string]struct {
		files map[string][]byte
		want  string
	}{
		"bad checksum":  {map[string][]byte{name: data, "checksums.txt": []byte(strings.Repeat("0", 64) + "  " + name)}, "checksum mismatch"},
		"no checksum":   {map[string][]byte{name: data, "checksums.txt": []byte(sum(data) + "  other.tar.gz")}, "no entry for " + name},
		"no checksums":  {map[string][]byte{name: data}, "no checksums.txt"},
		"no platform":   {map[string][]byte{"checksums.txt": nil}, "has no " + name},
		"wrong archive": {map[string][]byte{name: archive(t, "other", nil), "checksums.txt": []byte(sum(archive(t, "other", nil)) + "  " + name)}, "no ledge in the archive"},
	} {
		client := fakeGitHub(t, "v1.0.0", c.files)
		release, err := client.Latest()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Download(release); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", label, err, c.want)
		}
	}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledge")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	// Through a symlink, the file it points at is replaced and the link kept.
	if err := Replace(link, []byte("new")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "new" || info.Mode().Perm() != 0o755 {
		t.Errorf("after Replace: %q, mode %v", data, info.Mode())
	}
	if target, _ := os.Readlink(link); target != path {
		t.Errorf("the symlink now points at %q", target)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("left %d files in the directory, want 2", len(entries))
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		tag, current string
		want         bool
	}{
		{"v1.2.3", "v1.2.2", true},
		{"v1.10.0", "v1.9.9", true},
		{"v2.0.0", "1.99.99", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.3.0", false},
		{"v1.2.3", "v1.2.3-rc1", false},
	} {
		if got, err := Newer(c.tag, c.current); err != nil || got != c.want {
			t.Errorf("Newer(%s, %s) = %v, %v", c.tag, c.current, got, err)
		}
	}
	if _, err := Newer("latest", "v1.0.0"); err == nil {
		t.Error("a tag that isn't vX.Y.Z should be an error")
	}
}
