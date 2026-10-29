package components

import (
	"context"
	"io"
	"regexp"
	"strings"

	"github.com/a-h/templ"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTML is a report's block written as HTML, such as one rendered from Markdown. Only
// what's in policy is kept, so its markup is what a theme styles, and nothing else:
// no scripts, styles, classes, ids or other attributes, and no images but data:
// ones, as the page can't fetch anything and mustn't try. Text is shown as it would
// be in any page: runs of white space are one space, but in a <pre>.
//
// A few things it has are more than HTML:
//   - an <aside> is a note, as Note is, in its data-color: a theme's named colour or
//     #rrggbb, or gray for anything else, none included;
//   - a <table> scrolls sideways, as Table does;
//   - a <button data-action="name">, and a link to action:name, calls actions[name],
//     as a Buttons' button calls its Click, name being its id. A <button> is styled
//     as one of a Buttons'. One whose name isn't in actions, or has no Click, is
//     disabled, as is a button without data-action; such a link is aria-disabled
//     and goes nowhere.
func HTML(src string, actions map[string]Click) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
		nodes, err := html.ParseFragment(strings.NewReader(policy.Sanitize(src)), body)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			body.AppendChild(n)
		}
		rewrite(ctx, body, actions, false)
		for n := body.FirstChild; n != nil; n = n.NextSibling {
			if err := html.Render(w, n); err != nil {
				return err
			}
		}
		return nil
	})
}

// policy is what HTML keeps of what it's given.
var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "hr", "h1", "h2", "h3", "h4", "h5", "h6",
		"strong", "b", "em", "i", "u", "s", "del", "ins", "mark", "small", "sub", "sup",
		"code", "kbd", "samp", "pre", "blockquote", "ul", "ol", "li", "dl", "dt", "dd",
		"table", "caption", "thead", "tbody", "tfoot", "tr", "th", "td",
		"details", "summary", "aside", "button")
	p.AllowAttrs("start").Matching(bluemonday.Integer).OnElements("ol")
	p.AllowAttrs("reversed").OnElements("ol")
	p.AllowAttrs("colspan", "rowspan").Matching(bluemonday.Integer).OnElements("th", "td")
	p.AllowAttrs("open").OnElements("details")
	p.AllowAttrs("title").OnElements("abbr")
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowURLSchemes("http", "https", "action")
	p.RequireParseableURLs(true)
	p.AllowAttrs("src", "alt").OnElements("img")
	p.AllowAttrs("width", "height").Matching(bluemonday.Integer).OnElements("img")
	p.AllowDataURIImages()
	p.AllowAttrs("data-color").OnElements("aside")
	p.AllowAttrs("data-action", "disabled").OnElements("button")
	return p
}()

// space is a run of white space, as a page would show it: one space.
var space = regexp.MustCompile(`[ \t\n\f\r]+`)

// rewrite gives what's in n, sanitized, what HTML has more than HTML: its notes,
// tables and actions, with their Clicks put in ctx's Clicks. It collapses white space
// in text, but in a <pre>.
func rewrite(ctx context.Context, n *html.Node, actions map[string]Click, pre bool) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		switch {
		case c.Type == html.TextNode && !pre:
			c.Data = space.ReplaceAllString(c.Data, " ")
		case c.Type != html.ElementNode:
		case c.DataAtom == atom.Img && !strings.HasPrefix(attr(c, "src"), "data:image/"):
			n.RemoveChild(c)
			c = next
			continue
		case c.DataAtom == atom.Aside:
			color := attr(c, "data-color")
			setAttr(c, "class", "note")
			setAttr(c, "data-color", colorName(color))
			setAttr(c, "style", "--note: "+cssColor(color))
		case c.DataAtom == atom.Table:
			wrap := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div,
				Attr: []html.Attribute{{Key: "class", Val: "table"}}}
			n.InsertBefore(wrap, c)
			n.RemoveChild(c)
			wrap.AppendChild(c)
		case c.DataAtom == atom.Button:
			name := attr(c, "data-action")
			removeAttr(c, "data-action")
			setAttr(c, "type", "button")
			setAttr(c, "class", "button")
			if name == "" || !bind(ctx, c, name, actions) {
				setAttr(c, "disabled", "")
			}
		case c.DataAtom == atom.A && strings.HasPrefix(attr(c, "href"), "action:"):
			name := strings.TrimPrefix(attr(c, "href"), "action:")
			removeAttr(c, "href")
			if !bind(ctx, c, name, actions) {
				setAttr(c, "aria-disabled", "true")
			}
		}
		rewrite(ctx, c, actions, pre || c.DataAtom == atom.Pre)
		c = next
	}
}

// bind has n call actions[name], with the id ClickID gives it as name, and says
// whether there was one to call.
func bind(ctx context.Context, n *html.Node, name string, actions map[string]Click) bool {
	click := actions[name]
	if click == nil {
		return false
	}
	setAttr(n, "data-click", ClickID(ctx, name, click))
	return true
}

// attr is n's attribute key, or "".
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// setAttr sets n's attribute key to val, adding it if n hasn't one.
func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// removeAttr takes n's attribute key off it.
func removeAttr(n *html.Node, key string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}
