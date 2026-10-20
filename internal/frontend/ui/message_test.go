package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

func TestReadMessage(t *testing.T) {
	for body, want := range map[string]string{
		`{"action": "ready"}`:                            "",
		`{"action": "click", "id": "a"}`:                 "",
		`{"action": "click"}`:                            "click needs an id",
		`{"action": "ready", "id": "a"}`:                 "ready takes no id",
		`{"action": "open", "id": "a"}`:                  `no action "open"`,
		`{"action": "click", "id": "a", "path": "/etc"}`: `unknown field "path"`,
		`"click"`:                           "cannot unmarshal string",
		`{"action": "click", "id": "a"} {}`: "more than one value",
	} {
		_, err := readMessage([]byte(body))
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: got %v, want %q", body, err, want)
		}
	}
}

// Clicks post only ids, which reach the Clicks. An action with a Click is clicked
// apart from its card; one without is part of it. A click on what's gone is dropped.
func TestClicks(t *testing.T) {
	gui(t)
	show(t, []components.Basic{
		{ID: "card-open", Title: "Opens", Click: record("open card-open"), Dismiss: record("dismiss card-open"),
			Actions: []components.Action{
				{ID: "qa-run", Title: "Run", Click: record("run qa-run")},
				{ID: "qa-link", Title: "Link", Click: record("open qa-link")},
				{ID: "qa-plain", Title: "Plain"},
			}},
		{ID: "card-still", Title: "Still"},
	})
	waitFor(t, `document.querySelectorAll(".action").length`, "3")
	if got := eval(t, `document.querySelectorAll("#card-card-still .dismiss").length`); got != "0" {
		t.Error("a card without a Dismiss has ×")
	}
	eval(t, `document.getElementById("action-qa-run").click(), document.querySelector("#action-qa-link .label").click(), true`)
	for _, call := range []string{"run qa-run", "open qa-link"} {
		eventually(t, call, func() bool { return calls.has(call) })
	}
	time.Sleep(200 * time.Millisecond)
	if calls.has("open card-open") {
		t.Error("clicking an action also opened its card")
	}
	for _, js := range []string{
		`document.getElementById("action-qa-plain").click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Still")).click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Opens")).querySelector(".dismiss").click()`,
	} {
		eval(t, js+", true")
	}
	for _, call := range []string{"open card-open", "dismiss card-open"} {
		eventually(t, call, func() bool { return calls.has(call) })
	}
	time.Sleep(200 * time.Millisecond)
	for _, call := range []string{"open qa-plain", "open card-still", "open qa-run"} {
		if calls.has(call) {
			t.Errorf("clicking something that doesn't open called %q", call)
		}
	}

	// Once a render takes a card off, a click on it is dropped.
	before := logs.Len()
	testBar.Render(components.Cards([]components.Basic{{ID: "card-still", Title: "Still"}}))
	eval(t, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "action-qa-run"}), true`)
	eventually(t, "the click to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't on it") })
}

func TestMessagesAreChecked(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "kept", Title: "Kept", Click: record("click kept")}})
	waitFor(t, `document.getElementById("card-kept") !== null`, "true")
	before := logs.Len()
	for _, message := range []string{`{action: "click"}`, `{action: "click", id: "card-kept", path: "/etc"}`, `{action: "exec", id: "card-kept"}`, `"click"`} {
		eval(t, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	eventually(t, "4 refusals", func() bool { return strings.Count(logs.String()[before:], "the bar sent") == 4 })
	if calls.has("click kept") {
		t.Error("a refused message reached a Click")
	}
}

// A value that can't be JSON, posted or returned, is dropped or an error, not a crash.
func TestNotJSON(t *testing.T) {
	gui(t)
	eval(t, `[new Date(), NaN, {n: Infinity}].forEach(m => window.webkit.messageHandlers.ledge.postMessage(m)), true`)
	if _, err := shim.EvalResult(BarWindow, `new Date()`); err == nil {
		t.Error("a Date result should be an error")
	}
	if got := eval(t, `1 + 1`); got != "2" {
		t.Errorf("after values that aren't JSON, 1 + 1 = %s", got)
	}
}
