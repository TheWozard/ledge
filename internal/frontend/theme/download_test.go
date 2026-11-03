package theme

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// withCache has Cache an empty directory of its own for the test.
func withCache(t *testing.T) {
	t.Helper()
	old := Cache
	Cache = t.TempDir()
	t.Cleanup(func() { Cache = old })
}

// A theme's icons and fonts by url are downloaded into Cache, each once, and from
// then it loads with them inlined; one that can't be, or isn't what it should be, is
// an error. Until it's downloaded, an icon's button keeps its text and a font's url
// is left as it is, as it is again once ClearCache has deleted them.
func TestDownload(t *testing.T) {
	withCache(t)
	svg := `<svg viewBox="0 0 16 16"><path d="M0 0h16v16z"/></svg>`
	gets := map[string]int{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets[r.URL.Path]++
		switch r.URL.Path {
		case "/dots.svg":
			w.Write([]byte(svg))
		case "/inter.woff2":
			w.Write([]byte("wOF2 font"))
		case "/page.svg", "/page.woff2":
			w.Write([]byte("<html><body>Sign in</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	file := func(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }
	fonts := `@font-face { src: url("` + server.URL + `/inter.woff2") } @font-face { src: url(` + server.URL + `/page.woff2) }`
	fsys := fstest.MapFS{
		"cdn.json": file(`{"name": "cdn", "bar": ["fonts.css"], "report": ["fonts.css"], "icons": {"menu": "` + server.URL +
			`/dots.svg", "dismiss": "` + server.URL + `/page.svg", "copy": "` + server.URL + `/gone.svg"}}`),
		"fonts.css": file(fonts),
	}
	if th, err := Load(fsys, "cdn.json"); err != nil || th.Icons != (Icons{}) || th.Bar != fonts {
		t.Fatalf("before downloading, Load = %+v, %v; want no icons, and the fonts as they are", th, err)
	}
	set, err := ReadSet(fsys)
	if err != nil {
		t.Fatal(err)
	}
	set.Theme("cdn") // so the set has it as it is, until its files are downloaded

	downloaded, err := set.Download(context.Background(), server.Client(), "cdn")
	if err == nil {
		t.Error("an icon and a font that are a page, and an icon that isn't there, should be errors")
	}
	if want := []string{server.URL + "/dots.svg", server.URL + "/inter.woff2"}; !slices.Equal(downloaded, want) {
		t.Errorf("downloaded %q, want %q", downloaded, want)
	}
	th, err := set.Theme("cdn")
	if want := (Icons{Menu: iconDataURL([]byte(svg))}); err != nil || th.Icons != want {
		t.Errorf("after downloading, the icons are %+v, %v; want %+v", th.Icons, err, want)
	}
	want := `@font-face { src: url(data:font/woff2;base64,d09GMiBmb250) } @font-face { src: url(` + server.URL + `/page.woff2) }`
	if th.Bar != want || th.Report != want {
		t.Errorf("after downloading, the stylesheets are %s and %s, want %s", th.Bar, th.Report, want)
	}
	if downloaded, _ := Download(context.Background(), server.Client(), fsys, "cdn.json"); len(downloaded) != 0 ||
		gets["/dots.svg"] != 1 || gets["/inter.woff2"] != 1 {
		t.Errorf("again, it downloaded %q, and got %v; want what's downloaded kept, and the font once for both stylesheets", downloaded, gets)
	}
	if !strings.HasPrefix(cacheFile(server.URL+"/inter.woff2"), Cache) || !strings.HasSuffix(cacheFile(server.URL+"/inter.woff2"), ".woff2") {
		t.Errorf("the font is cached at %s", cacheFile(server.URL+"/inter.woff2"))
	}

	// Cleared, the theme loads as it did before anything was downloaded.
	if cleared, err := ClearCache(); !cleared || err != nil {
		t.Errorf("ClearCache = %v, %v; want it cleared", cleared, err)
	}
	if th, err := Load(fsys, "cdn.json"); err != nil || th.Icons != (Icons{}) || th.Bar != fonts {
		t.Errorf("after clearing, Load = %+v, %v; want no icons, and the fonts as they are", th, err)
	}
	if cleared, err := ClearCache(); cleared || err != nil {
		t.Errorf("ClearCache again = %v, %v; want nothing to clear", cleared, err)
	}
}

// An icon is a relative path or an https url; any other url is an error.
func TestIconURLs(t *testing.T) {
	for _, icon := range []string{"http://example.com/x.svg", "file:///x.svg", "data:image/svg+xml,<svg/>", "https:x.svg"} {
		if _, err := readManifest([]byte(`{"name": "x", "icons": {"menu": "` + icon + `"}}`)); err == nil {
			t.Errorf("icon %q should be an error", icon)
		}
	}
	for _, icon := range []string{"icons/x.svg", "../x.svg", "https://cdn.example.com/x@1.0/x.svg"} {
		if _, err := readManifest([]byte(`{"name": "x", "icons": {"menu": "` + icon + `"}}`)); err != nil {
			t.Errorf("icon %q: %v", icon, err)
		}
	}
}
