package components

import (
	"testing"
	"time"
)

// A message shows its avatar, sender and when it was sent on its first line, then its
// title and description, all as text.
func TestMessage(t *testing.T) {
	sent := time.Now()
	card := Message{Basic: Basic{ID: "dm", Title: "<Re: deploy>", Description: "Looks good, <b>ship it</b>"},
		From: "Ada Lovelace", Sent: sent}
	html := render(t, BarContent([]Card{card}, nil))
	contains(t, html,
		`<li id="card-dm" class="message">`,
		`<div class="message-head"><span class="avatar" aria-hidden="true">AL</span> <span class="from">Ada Lovelace</span> `+
			`<time class="sent" datetime="`+sent.Format(time.RFC3339)+`">`+sent.Format(time.Kitchen)+`</time></div>`,
		`<span class="title">&lt;Re: deploy&gt;</span>`,
		`<div class="description">Looks good, &lt;b&gt;ship it&lt;/b&gt;</div>`)
	lacks(t, html, "<b>")

	// Without a time or a title, neither shows.
	bare := render(t, BarContent([]Card{Message{Basic: Basic{ID: "b"}, From: "bot"}}, nil))
	lacks(t, bare, "<time", `class="title"`, `class="description"`)
}

// An avatar is shown only as a data: URL of an image; anything else, which the page
// would try to fetch, or could run, is the sender's initials instead.
func TestAvatar(t *testing.T) {
	const png = "data:image/png;base64,iVBORw0KGgo="
	html := render(t, BarContent([]Card{Message{Basic: Basic{ID: "a"}, From: "Ada", Avatar: png}}, nil))
	contains(t, html, `<img class="avatar" src="`+png+`" alt="">`)
	for _, avatar := range []string{
		"https://example.com/ada.png",
		"javascript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		`data:image/png;base64,AAAA" onerror="alert(1)`,
		"data:image/svg+xml,<svg onload=alert(1)>",
	} {
		html := render(t, BarContent([]Card{Message{Basic: Basic{ID: "a"}, From: "Ada", Avatar: avatar}}, nil))
		contains(t, html, `<span class="avatar" aria-hidden="true">A</span>`)
		lacks(t, html, "<img", "example.com", "alert")
	}
}

func TestInitials(t *testing.T) {
	for name, want := range map[string]string{
		"Ada Lovelace": "AL", "ada": "A", "Ada King Lovelace": "AK", "  ": "?", "": "?",
		"@ci-bot": "C", "émile zola": "ÉZ", "李 小龙": "李小", "#general team": "GT",
	} {
		if got := initials(name); got != want {
			t.Errorf("initials(%q) = %q, want %q", name, got, want)
		}
	}
}

// A message's time is the time if it was sent today, the day and time within the
// week, and the date before that.
func TestSentAt(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	for sent, want := range map[time.Time]string{
		now: "3:00PM",
		time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC):  "12:00AM",
		now.Add(-16 * time.Hour):                       "Mon 11:00PM",
		time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC): "Wed 9:30AM",
		time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC): "Oct 13",
		// Shown in now's time zone.
		time.Date(2026, 10, 20, 10, 0, 0, 0, time.FixedZone("EST", -5*3600)): "3:00PM",
	} {
		if got := sentAt(sent, now); got != want {
			t.Errorf("sentAt(%v) = %q, want %q", sent, got, want)
		}
	}
}
