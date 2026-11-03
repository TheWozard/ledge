package ui

import (
	"sync"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
)

// A card that ends has its Complete called once its end passes, once for that end
// however many renders show it, and not at all if it's gone by then. It needs no
// window: a bar whose page isn't ready keeps what it's given.
func TestComplete(t *testing.T) {
	bar := newBar(Options{}, &fakeNative{})
	var mu sync.Mutex
	counts := map[string]int{}
	timer := func(id string, end time.Time) components.Timer {
		return components.Timer{Basic: components.Basic{ID: id, Title: id}, Start: time.Now(), End: end,
			Complete: func() { mu.Lock(); counts[id]++; mu.Unlock() }}
	}
	count := func(id string) int { mu.Lock(); defer mu.Unlock(); return counts[id] }

	soon, gone, past := timer("soon", time.Now().Add(100*time.Millisecond)), timer("gone", time.Now().Add(100*time.Millisecond)), timer("past", time.Now().Add(-time.Second))
	bar.Render([]components.Card{soon, gone, past})
	eventually(t, "past to complete at once", func() bool { return count("past") == 1 })
	// Rendering again before soon ends doesn't make it complete twice, and gone's left.
	bar.Render([]components.Card{soon, past})
	eventually(t, "soon to complete", func() bool { return count("soon") == 1 })
	bar.Render([]components.Card{soon, past})
	time.Sleep(200 * time.Millisecond)
	if count("soon") != 1 || count("past") != 1 || count("gone") != 0 {
		t.Errorf("completed %v, want soon and past once and gone not at all", counts)
	}
	// A new end is a new completion.
	bar.Render([]components.Card{timer("soon", time.Now())})
	eventually(t, "soon to complete at its new end", func() bool { return count("soon") == 2 })
}
