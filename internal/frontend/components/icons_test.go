package components

import (
	"context"
	"strings"
	"testing"
)

// A theme's icons go on the bar's and report's pages as CSS variables, between the layout and the
// theme, and a button with one shows it rather than its text; one without, or with
// what isn't an SVG data: url, shows its text.
func TestIcons(t *testing.T) {
	icon := "data:image/svg+xml;base64,PHN2Zy8+"
	page := render(t, BarPage("/* theme */", "top", Icons{Menu: icon, Dismiss: `x"); background: red`}))
	contains(t, page, "<style>\n:root {\n  --icon-menu: url(\""+icon+"\");\n}\n</style><style>\n/* theme */</style>")
	lacks(t, page, "--icon-dismiss: url", "background: red")
	lacks(t, render(t, BarPage("", "top", Icons{})), ":root {\n  --icon-")
	contains(t, render(t, ReportPage("/* theme */", Icons{Copy: icon})), "<style>\n:root {\n  --icon-copy: url(\""+icon+"\");\n}\n</style><style>\n/* theme */</style>")

	card := Basic{ID: "c", Title: "C", Dismiss: nothing, Actions: []Action{{ID: "a", Title: "A", Click: nothing}}}
	var html string
	html, _ = renderClicks(t, BarContent([]Card{card}, nil))
	contains(t, html, `data-click="card/c/menu">⋯</button>`, `data-click="card/c/dismiss">×</button>`)
	lacks(t, html, `class="icon"`)

	var out strings.Builder
	ctx := WithIcons(WithClicks(context.Background(), &Clicks{}), Icons{Menu: icon, Dismiss: "x.svg"})
	if err := BarContent([]Card{card}, nil).Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	contains(t, out.String(), `data-click="card/c/menu"><span class="icon" aria-hidden="true"></span></button>`,
		`data-click="card/c/dismiss">×</button>`)
	lacks(t, out.String(), "⋯")
}
