package components

import (
	"regexp"
	"slices"
)

// named is the colours a theme names, each its --c-<name>.
var named = []string{"green", "yellow", "red", "blue", "gray"}

var hex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// colorName is a colour as the page names it in data-color: a named one, #rrggbb, or
// gray for anything else, "" included.
func colorName(color string) string {
	if slices.Contains(named, color) || hex.MatchString(color) {
		return color
	}
	return "gray"
}

// cssColor is a colour as CSS: a named one is the theme's variable for it, and
// anything that isn't a colour is gray's.
func cssColor(color string) string {
	if color = colorName(color); hex.MatchString(color) {
		return color
	}
	return "var(--c-" + color + ")"
}
