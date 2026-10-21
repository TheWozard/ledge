package components

import (
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Each block shows its text escaped; a table's rows line up with its columns, and a
// link that isn't safe goes nowhere.
func TestBlocks(t *testing.T) {
	blocks := []templ.Component{
		Heading("Jobs"),
		Paragraph("<b>not bold</b>\nsecond line"),
		Fields([]Field{{Label: "Branch", Value: "main"}}),
		Table([]string{"Job", "Result"}, [][]string{{"lint", "ok"}, {"test"}, {"build", "ok", "extra"}}),
		List([]string{"one", "two"}, true),
		Code("$ go test\nFAIL"),
		Note("Flaky", "yellow"),
		Note("Plain", ""),
		Note("Purple", "purple"),
		Link("The run", "https://example.com/run"),
		Link("Bad", "javascript:alert(1)"),
	}
	var html strings.Builder
	for _, block := range blocks {
		html.WriteString(render(t, block))
	}
	contains(t, html.String(),
		"<h2>Jobs</h2>", "<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line</p>",
		`<div class="field"><dt>Branch</dt><dd>main</dd></div>`,
		"<th>Job</th><th>Result</th>",
		"<tr><td>test</td><td></td></tr>", "<tr><td>build</td><td>ok</td></tr>",
		"<ol><li>one</li><li>two</li></ol>", "<pre><code>$ go test\nFAIL</code></pre>",
		`data-color="yellow" style="--note: var(--c-yellow);"`, `data-color="gray" style="--note: var(--c-gray);">Plain`,
		`data-color="gray" style="--note: var(--c-gray);">Purple`,
		`<a href="https://example.com/run" title="https://example.com/run">The run</a>`)
	lacks(t, html.String(), "<b>not bold", "extra", `href="javascript:`)
}

// A block with an id comes in a template with it, its text escaped.
func TestBlock(t *testing.T) {
	html := render(t, Block(`"x"`, Paragraph("<b>")))
	contains(t, html, `<template data-block="&#34;x&#34;"><p>&lt;b&gt;</p></template>`)
}
