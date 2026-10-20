// Package components is the bar's and the report window's HTML, as templ components:
// the pages (BarPage, ReportPage), what goes in them (BarContent, and the report
// blocks: Heading, Paragraph, Fields, Table, List, Code, Note, Link), and the data
// they show (Card, Action).
//
// Each kind of card has a file of its own (basic.templ, timer.templ), with its type
// and its body. The pages' layout styles and scripts are in page/.
//
// Every string given to a component is shown as text: templ escapes it, so event and
// report text can't add markup. A card's colour is #rrggbb; anything else is none.
// An action's or a note's is a theme's named colour (green, yellow, red, blue,
// gray) or #rrggbb; anything else shows as gray.
package components
