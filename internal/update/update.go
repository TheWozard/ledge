// Package update replaces the running ledge binary with the latest GitHub release.
// Releases are built by GoReleaser (.goreleaser.yaml): one ledge_<os>_<arch>.tar.gz
// per platform, and a checksums.txt of their SHA-256 sums.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// Owner and Repo name the GitHub repository releases come from.
	Owner = "TheWozard"
	Repo  = "ledge"
	// Binary is the binary's name, in the archive and installed.
	Binary = "ledge"
	// DefaultAPI is GitHub's API. $LEDGE_GITHUB_API replaces it, for testing against
	// a local copy.
	DefaultAPI = "https://api.github.com"
)

// Release is a GitHub release: its tag and its assets.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Asset is one file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Client fetches releases.
type Client struct {
	API  string // GitHub's API, or a stand-in
	HTTP *http.Client
}

// NewClient talks to $LEDGE_GITHUB_API, or GitHub.
func NewClient() *Client {
	api := os.Getenv("LEDGE_GITHUB_API")
	if api == "" {
		api = DefaultAPI
	}
	return &Client{API: strings.TrimRight(api, "/"), HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

// AssetName is the archive for this platform.
func AssetName() string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", Binary, runtime.GOOS, runtime.GOARCH)
}

// Latest is the newest release.
func (c *Client) Latest() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.API, Owner, Repo)
	body, err := c.get(url, "application/vnd.github+json")
	if err != nil {
		return nil, fmt.Errorf("checking the latest release: %w", err)
	}
	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("checking the latest release: %w", err)
	}
	if release.Tag == "" {
		return nil, errors.New("checking the latest release: no tag in the reply")
	}
	return &release, nil
}

// Download fetches this platform's binary from release, checked against the
// release's checksums.txt. A release without a checksum for it is refused.
func (c *Client) Download(release *Release) ([]byte, error) {
	name := AssetName()
	archive := release.asset(name)
	if archive == nil {
		return nil, fmt.Errorf("%s has no %s (only macOS on arm64 and amd64 is released)", release.Tag, name)
	}
	sums := release.asset("checksums.txt")
	if sums == nil {
		return nil, fmt.Errorf("%s has no checksums.txt to check %s against", release.Tag, name)
	}
	data, err := c.get(archive.URL, "")
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", name, err)
	}
	list, err := c.get(sums.URL, "")
	if err != nil {
		return nil, fmt.Errorf("downloading checksums.txt: %w", err)
	}
	if err := checkSum(list, name, data); err != nil {
		return nil, err
	}
	binary, err := extract(data, Binary)
	if err != nil {
		return nil, fmt.Errorf("unpacking %s: %w", name, err)
	}
	return binary, nil
}

func (r *Release) asset(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

func (c *Client) get(url, accept string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func checkSum(list []byte, name string, data []byte) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(string(list), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			if fields[0] != got {
				return fmt.Errorf("checksum mismatch for %s: checksums.txt has %s, the download is %s", name, fields[0], got)
			}
			return nil
		}
	}
	return fmt.Errorf("checksums.txt has no entry for %s", name)
}

func extract(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	files := tar.NewReader(gz)
	for {
		header, err := files.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("no %s in the archive", name)
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == name {
			return io.ReadAll(files)
		}
	}
}

// Replace swaps the file at path for binary, keeping its mode. It writes beside it
// and renames, so the file is never half-written. A ledge that's running keeps the old
// code until it's stopped.
func Replace(path string, binary []byte) error {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ledge-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // does nothing once renamed into place
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Newer reports whether tag is a later version than current. Both are vX.Y.Z (the
// "v" optional); anything after the patch number, such as -rc1, is ignored.
func Newer(tag, current string) (bool, error) {
	a, err := parse(tag)
	if err != nil {
		return false, err
	}
	b, err := parse(current)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return false, nil
}

func parse(version string) ([3]int, error) {
	var parts [3]int
	core, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return parts, fmt.Errorf("version %q isn't vX.Y.Z", version)
	}
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 {
			return parts, fmt.Errorf("version %q isn't vX.Y.Z", version)
		}
		parts[i] = n
	}
	return parts, nil
}
