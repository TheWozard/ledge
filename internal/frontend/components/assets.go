package components

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"strings"

	"github.com/a-h/templ"
)

// The pages' own layout styles and scripts, inlined into them, since they can't
// fetch files. The layout styles' opening comments say what a theme can hook into.
var (
	//go:embed page/ledge.js
	ledgeJS string
	//go:embed page/bar.css
	barCSS string
	//go:embed page/bar.js
	barJS string
	//go:embed page/report.css
	reportCSS string
	//go:embed page/report.js
	reportJS string
	//go:embed page/menu.css
	menuCSS string
	//go:embed page/menu.js
	menuJS string
)

// idiomorph is Idiomorph 0.8.0 (github.com/bigskysoftware/idiomorph, 0BSD; see
// vendor/idiomorph.LICENSE), dist/idiomorph.min.js from npm. The bar's page inlines
// it to morph what it shows rather than replace it.
//
//go:embed vendor/idiomorph.min.js
var idiomorph string

// style is css inlined in a <style>, as it's written.
func style(css string) templ.Component { return templ.Raw("<style>\n" + css + "</style>") }

// script is js inlined in a <script>, as it's written.
func script(js string) templ.Component { return templ.Raw("<script>" + js + "</script>") }

// Each page's scripts, in the order it runs them, and its csp, which lets only these
// run, worked out once.
var (
	barScripts    = []string{idiomorph, ledgeJS, barJS}
	reportScripts = []string{idiomorph, ledgeJS, reportJS}
	menuScripts   = []string{ledgeJS, menuJS}
	barCSP        = csp(barScripts)
	reportCSP     = csp(reportScripts)
	menuCSP       = csp(menuScripts)
)

// csp is a page's Content-Security-Policy, first in its <head>: it runs only scripts,
// each by its hash, so nothing else can, not a <script> nor an onclick in what it's
// shown, should any get past templ's escaping. Styles are inline, and only
// data: images and fonts load; the page fetches nothing else. The app's own calls,
// such as update, aren't the page's scripts, and aren't stopped by it.
func csp(scripts []string) templ.Component {
	sources := []string{"script-src"}
	for _, js := range scripts {
		sum := sha256.Sum256([]byte(js))
		sources = append(sources, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	policy := "default-src 'none'; " + strings.Join(sources, " ") +
		"; style-src 'unsafe-inline'; img-src data:; font-src data:; base-uri 'none'; form-action 'none'"
	return templ.Raw(`<meta http-equiv="Content-Security-Policy" content="` + policy + `"/>`)
}
