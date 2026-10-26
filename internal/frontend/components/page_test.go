package components

import (
	"testing"

	"github.com/a-h/templ"
)

func TestBarPage(t *testing.T) {
	html := render(t, BarPage(":root { --bar: #fff; }", "top"))
	contains(t, html, `<body class="top">`, "<style>\n:root { --bar: #fff; }</style>", `<div id="app"></div>`,
		// The layout and the script come through as written.
		"/* Layout only. How the bar looks comes from its theme", `send("ready");`, `event.target.closest("[data-click]")`,
		// Idiomorph is inlined whole, and update morphs with it.
		"<script>var Idiomorph=", `Idiomorph.morph(app, html, {`)
}

// The report page draws its title bar and an empty <article>, under its theme, with
// its script, which morphs what it's given into the <article>.
func TestReportPage(t *testing.T) {
	html := render(t, ReportPage(":root { --page: #fff; }"))
	contains(t, html,
		`<header class="titlebar"><span></span>`, `<button class="close"`,
		"<style>\n:root { --page: #fff; }</style>",
		"<article></article>",
		`frame("drag")`, "<script>var Idiomorph=", `Idiomorph.morph(article, html, {`)
}

// A report's content is its blocks, top to bottom, and nothing else.
func TestReportContent(t *testing.T) {
	html := render(t, ReportContent([]templ.Component{Heading("Jobs"), Block{ID: "x", Content: Paragraph("CI <failed>")}}))
	if want := "<h2>Jobs</h2><p>CI &lt;failed&gt;</p>"; html != want {
		t.Errorf("the report's content is %s, want %s", html, want)
	}
}
