package theme

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
)

// A theme can name files it can't ship itself, such as icons or fonts whose licence
// doesn't allow it, by their https urls: an icon in its manifest, and a font in a
// stylesheet, url(https://…/inter.woff2). Each user downloads them for themselves,
// with Download (the cache-theme command), into Cache, and Load reads them from there,
// inlined as any other. Until then, or if one's gone, an icon's button shows its text,
// and a font's url is left as it is, which a page can't fetch, so the stylesheet's
// next font is used instead.

// Cache is the directory the files a theme names by url are downloaded into, by
// Download, and read from, by Load: ledge/themes in the user's cache directory, such
// as ~/Library/Caches/ledge/themes, or "" if there's none, and then none is ever
// used. Set it before loading any theme to use another.
var Cache = func() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ledge", "themes")
}()

// maxDownload is the most a file's download can be: an icon is a few hundred bytes,
// a font tens or hundreds of kilobytes.
const maxDownload = 16 << 20

// remoteFontURL is how a stylesheet names a font to download: url(<https url>), its
// url quoted or not, to a woff2, woff, ttf or otf.
var remoteFontURL = regexp.MustCompile(`url\(\s*["']?(https://[^"'()\s]+\.(woff2|woff|ttf|otf))["']?\s*\)`)

// fontMagic is how each kind of font file starts.
var fontMagic = map[string][][]byte{
	"woff2": {[]byte("wOF2")},
	"woff":  {[]byte("wOFF")},
	"ttf":   {{0, 1, 0, 0}, []byte("true")},
	"otf":   {[]byte("OTTO")},
}

// isRemote is whether a file a theme names is an https url, rather than a path.
func isRemote(name string) bool {
	u, err := url.Parse(name)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// cacheFile is where in Cache the file at url is: named by its hash, so each url, a
// new version of an icon set's or a font's included, has its own, with its
// extension.
func cacheFile(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(Cache, hex.EncodeToString(sum[:])+path.Ext(url))
}

// cached is the file at url, from Cache, if it's been downloaded.
func cached(url string) ([]byte, bool) {
	if Cache == "" {
		return nil, false
	}
	data, err := os.ReadFile(cacheFile(url))
	return data, err == nil
}

// cachedIcon is the icon at url as a data: url, from Cache, or "" if it isn't there
// or isn't an SVG, for its button to show its text.
func cachedIcon(url string) string {
	data, ok := cached(url)
	if !ok || !isSVG(data) {
		return ""
	}
	return iconDataURL(data)
}

// inlineRemoteFonts replaces each font url in css that's been downloaded with it, as
// a data: url, and leaves the rest.
func inlineRemoteFonts(css string) string {
	return remoteFontURL.ReplaceAllStringFunc(css, func(url string) string {
		match := remoteFontURL.FindStringSubmatch(url)
		data, ok := cached(match[1])
		if !ok {
			return url
		}
		return fontDataURL(match[2], data)
	})
}

// Download downloads, with client, each file the theme whose manifest is file in
// fsys names by url, its icons and its stylesheets' fonts, into Cache, for Load to
// use from then. A file already there is left as it is. It returns the urls it
// downloaded; one that can't be, such as an icon that isn't an SVG, is an error, and
// the rest are still downloaded.
func Download(ctx context.Context, client *http.Client, fsys fs.FS, file string) ([]string, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("theme: %w", err)
	}
	manifest, err := readManifest(data)
	if err != nil {
		return nil, fmt.Errorf("theme %s: %w", file, err)
	}
	if Cache == "" {
		return nil, errors.New("theme: there's no cache directory to download into")
	}
	remote, err := remoteFiles(fsys, path.Dir(file), manifest)
	if err != nil {
		return nil, fmt.Errorf("theme %s: %w", manifest.Name, err)
	}
	var downloaded []string
	var errs []error
	for _, r := range remote {
		if _, ok := cached(r.url); ok {
			continue
		}
		if err := download(ctx, client, r.url, r.check); err != nil {
			errs = append(errs, fmt.Errorf("theme %s: %w", manifest.Name, err))
			continue
		}
		downloaded = append(downloaded, r.url)
	}
	return downloaded, errors.Join(errs...)
}

// ClearCache deletes everything downloaded into Cache, for every theme; a theme
// loaded after shows its icons' text and its fonts' fallbacks until its files are
// downloaded again. It reports whether there was anything to delete.
func ClearCache() (bool, error) {
	if Cache == "" {
		return false, nil
	}
	if _, err := os.Stat(Cache); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return true, os.RemoveAll(Cache)
}

// Download is Download for the theme name, which then loads again, with what's
// downloaded, the next time it's asked for.
func (s *Set) Download(ctx context.Context, client *http.Client, name string) ([]string, error) {
	file, ok := s.files[name]
	if !ok {
		return nil, fmt.Errorf("no theme %q", name)
	}
	downloaded, err := Download(ctx, client, s.fsys, file)
	if len(downloaded) > 0 {
		s.themes.Delete(name)
	}
	return downloaded, err
}

// remoteFile is a file a theme names by url, and what it must be: check says why
// what's downloaded isn't, or nil.
type remoteFile struct {
	url   string
	check func(data []byte) error
}

// remoteFiles is each file manifest, from dir in fsys, names by url, once: its icons,
// then its stylesheets' fonts, in order.
func remoteFiles(fsys fs.FS, dir string, manifest Manifest) ([]remoteFile, error) {
	var files []remoteFile
	add := func(f remoteFile) {
		if !slices.ContainsFunc(files, func(g remoteFile) bool { return g.url == f.url }) {
			files = append(files, f)
		}
	}
	for _, icon := range manifest.Icons.all() {
		if isRemote(*icon) {
			add(remoteFile{*icon, checkSVG})
		}
	}
	for _, sheets := range [][]string{manifest.Bar, manifest.Report, manifest.Menu} {
		for _, sheet := range sheets {
			css, err := fs.ReadFile(fsys, path.Join(dir, sheet))
			if err != nil {
				return nil, err
			}
			for _, match := range remoteFontURL.FindAllStringSubmatch(string(css), -1) {
				add(remoteFile{match[1], checkFont(match[2])})
			}
		}
	}
	return files, nil
}

// checkSVG is why data isn't an SVG, or nil.
func checkSVG(data []byte) error {
	if !isSVG(data) {
		return errors.New("isn't an SVG")
	}
	return nil
}

// checkFont is a check that data is a font of kind, such as woff2.
func checkFont(kind string) func(data []byte) error {
	return func(data []byte) error {
		for _, magic := range fontMagic[kind] {
			if bytes.HasPrefix(data, magic) {
				return nil
			}
		}
		return fmt.Errorf("isn't a %s font", kind)
	}
}

// download downloads the file at url into Cache, whole or not at all, if check
// passes it.
func download(ctx context.Context, client *http.Client, url string, check func([]byte) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	switch {
	case err != nil:
		return fmt.Errorf("%s: %w", url, err)
	case len(data) > maxDownload:
		return fmt.Errorf("%s is over %d bytes", url, maxDownload)
	}
	if err := check(data); err != nil {
		return fmt.Errorf("%s %w", url, err)
	}
	if err := os.MkdirAll(Cache, 0o755); err != nil {
		return err
	}
	// Written beside it, then moved into place, so a half-written file is never read.
	tmp, err := os.CreateTemp(Cache, ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), cacheFile(url))
}
