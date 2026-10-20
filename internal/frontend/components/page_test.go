package components

import (
	"testing"

	"github.com/a-h/templ"
)

func TestBarPage(t *testing.T) {
	html := render(t, BarPage(":root { --bar: #fff; }", "top"))
	contains(t, html, `<body class="top">`, "<style>\n:root { --bar: #fff; }</style>", `<div id="app"></div>`,
		// The layout and the script come through as written.
		"html, body { margin: 0;", `send("ready");`, `event.target.closest("[data-click]")`,
		// Idiomorph is inlined whole, and update morphs with it.
		"<script>var Idiomorph=", `Idiomorph.morph(app, html, {`)
}

// The report page draws its title bar, escaping the title, and the blocks in its
// <article>, under its theme, with its script.
func TestReportPage(t *testing.T) {
	html := render(t, ReportPage(":root { --page: #fff; }", "CI <failed>", []templ.Component{Heading("Jobs")}))
	contains(t, html,
		`<span>CI &lt;failed&gt;</span>`, `<button class="close"`,
		"<style>\n:root { --page: #fff; }</style>",
		"<article><h2>Jobs</h2></article>",
		`frame("drag")`)
}
