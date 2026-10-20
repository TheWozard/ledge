package components

import (
	_ "embed"

	"github.com/a-h/templ"
)

// The pages' own layout styles and scripts, inlined into them, since they can't
// fetch files. The layout styles' opening comments say what a theme can hook into.
var (
	//go:embed page/bar.css
	barCSS string
	//go:embed page/bar.js
	barJS string
	//go:embed page/report.css
	reportCSS string
	//go:embed page/report.js
	reportJS string
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
