package components

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	if err := c.Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// renderClicks renders c with Clicks, and returns them with the html.
func renderClicks(t *testing.T, c templ.Component) (string, *Clicks) {
	t.Helper()
	var out strings.Builder
	clicks := &Clicks{}
	if err := c.Render(WithClicks(context.Background(), clicks), &out); err != nil {
		t.Fatal(err)
	}
	return out.String(), clicks
}

func contains(t *testing.T, html string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
}

func lacks(t *testing.T, html string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(html, u) {
			t.Errorf("unexpected %q in:\n%s", u, html)
		}
	}
}

// nothing is a Click for a test that only renders.
func nothing(ClickEvent) {}
