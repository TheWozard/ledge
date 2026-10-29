package components

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestBarPage(t *testing.T) {
	html := render(t, BarPage(":root { --bar: #fff; }", "top", Icons{}))
	contains(t, html, `<body class="top">`, "<style>\n:root { --bar: #fff; }</style>", `<div id="app"></div>`,
		// The layout and the script come through as written.
		"/* Layout only. How the bar looks comes from its theme", `send("ready");`, `event.target.closest("[data-click]")`,
		// Idiomorph is inlined whole, and update morphs with it.
		"<script>var Idiomorph=", `Idiomorph.morph(app, html, {`)
}

// The report page draws its title bar and an empty <article>, under its theme, with
// its script, which morphs what it's given into the <article>.
func TestReportPage(t *testing.T) {
	html := render(t, ReportPage(":root { --page: #fff; }", Icons{}))
	contains(t, html,
		`<header class="titlebar"><span></span>`, `<button class="close"`,
		"<style>\n:root { --page: #fff; }</style>",
		"<article></article>",
		`frame("drag")`, "<script>var Idiomorph=", `Idiomorph.morph(article, html, {`)
}

// A report's content is its blocks, top to bottom, and nothing else.
func TestReportContent(t *testing.T) {
	html := render(t, ReportContent([]templ.Component{Heading(2, Text("Jobs")), Block{ID: "x", Content: Paragraph(Text("CI <failed>"))}}))
	if want := "<h2>Jobs</h2><p>CI &lt;failed&gt;</p>"; html != want {
		t.Errorf("the report's content is %s, want %s", html, want)
	}
}

// Each page's Content-Security-Policy comes first in its <head>, and lets run exactly
// the scripts the page has, by their hashes, and nothing else.
func TestCSP(t *testing.T) {
	scriptTag := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	hashSource := regexp.MustCompile(`'sha256-[^']+'`)
	for name, page := range map[string]templ.Component{
		"bar":    BarPage("", "top", Icons{}),
		"report": ReportPage("", Icons{}),
		"menu":   MenuPage("", []MenuItem{{Title: "Rerun", Enabled: true}}),
	} {
		html := render(t, page)
		policy, ok := strings.CutPrefix(html, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="`)
		if !ok {
			t.Errorf("%s: the policy isn't first in the <head>:\n%.300s", name, html)
			continue
		}
		policy, _, _ = strings.Cut(policy, `"`)
		var want []string
		for _, m := range scriptTag.FindAllStringSubmatch(html, -1) {
			sum := sha256.Sum256([]byte(m[1]))
			want = append(want, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
		if got := hashSource.FindAllString(policy, -1); len(want) == 0 || !slices.Equal(got, want) {
			t.Errorf("%s: the policy lets %q run, but the page's scripts are %q", name, got, want)
		}
		contains(t, policy, "default-src 'none'; script-src 'sha256-", "; style-src 'unsafe-inline'; img-src data:; font-src data:")
		lacks(t, policy, "unsafe-eval", "script-src 'unsafe-inline'")
	}
}
