// Package pages holds the HTML pages ledge shows, built into the binary. They decide
// how everything looks; Go passes data in and takes back actions that carry ids only.
package pages

import _ "embed"

// Bar is the bar along the edge of the screen.
//
//go:embed bar.html
var Bar string
