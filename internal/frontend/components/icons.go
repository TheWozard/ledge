package components

import (
	"context"
	"regexp"
	"strings"

	"github.com/a-h/templ"
)

// Icons is a theme's pictures for the bar's and report's buttons, each a data: url of
// an SVG, as theme.Icons has them, or "" for the button's text. Anything else is ""
// too.
type Icons struct {
	Menu    string // a card's ⋯
	Dismiss string // a card's ×
	Copy    string // a Code block's Copy, in a report
	Run     string // a RunBlock's button
	Stop    string // a RunBlock's stop button
}

// named is each of the icons, by the name of its CSS variable, --icon-<name>.
func (icons *Icons) named() []struct {
	name string
	url  *string
} {
	return []struct {
		name string
		url  *string
	}{{"menu", &icons.Menu}, {"dismiss", &icons.Dismiss}, {"copy", &icons.Copy}, {"run", &icons.Run}, {"stop", &icons.Stop}}
}

// iconURL is what an icon must be: an SVG as a base64 data: url, safe as it is in CSS.
var iconURL = regexp.MustCompile(`^data:image/svg\+xml;base64,[A-Za-z0-9+/]+={0,2}$`)

// valid is icons with any that isn't an iconURL taken out.
func (icons Icons) valid() Icons {
	for _, icon := range icons.named() {
		if !iconURL.MatchString(*icon.url) {
			*icon.url = ""
		}
	}
	return icons
}

// iconsKey is the context key of the icons buttons are drawn with.
type iconsKey struct{}

// WithIcons is ctx with icons, for BarContent and a report's Code blocks to draw the
// buttons that have one with it, rather than their text. The page must have been
// rendered with the same icons.
func WithIcons(ctx context.Context, icons Icons) context.Context {
	return context.WithValue(ctx, iconsKey{}, icons.valid())
}

// iconsIn is the icons ctx has, or none.
func iconsIn(ctx context.Context) Icons {
	icons, _ := ctx.Value(iconsKey{}).(Icons)
	return icons
}

// iconStyle is the icons as CSS, each as a variable on :root, such as --icon-menu,
// for a page's layout styles to draw them with; "" for none.
func iconStyle(icons Icons) templ.Component {
	icons = icons.valid()
	var vars []string
	for _, icon := range icons.named() {
		if *icon.url != "" {
			vars = append(vars, "--icon-"+icon.name+": url(\""+*icon.url+"\");")
		}
	}
	if len(vars) == 0 {
		return templ.NopComponent
	}
	return style(":root {\n  " + strings.Join(vars, "\n  ") + "\n}\n")
}
