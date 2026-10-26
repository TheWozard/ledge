package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/TheWozard/ledge/internal/store"
)

// shortTempDir is a temporary directory with a short path, unlike t.TempDir(): a
// socket's path must fit in 104 bytes.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// start runs a headless ledge of dir, and waits for its API to answer.
func start(t *testing.T, ctx context.Context, dir string) (api.Client, chan error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, dir, api.Options{Headless: true}) }()
	client := api.Client{Socket: state.Socket(dir)}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		var status api.Status
		if client.Call("GET", "/status", nil, &status) == nil {
			if !status.Options.Headless {
				t.Errorf("status = %+v", status)
			}
			return client, done
		}
		select {
		case err := <-done:
			t.Fatalf("Run returned before its API answered: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the API never answered")
		}
	}
}

func wait(t *testing.T, done chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return")
	}
}

func TestRun(t *testing.T) {
	dir := shortTempDir(t)
	// An event that expired while ledge wasn't running goes when it starts.
	events, err := store.Open(filepath.Join(dir, "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := events.Add(map[string]any{"id": "old", "title": "Old", "expires": "2020-01-01T00:00:00Z"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	client, done := start(t, context.Background(), dir)
	var added struct {
		ID string `json:"id"`
	}
	if err := client.Call("POST", "/events", []byte(`{"id": "new", "title": "New", "expires": "1s"}`), &added); err != nil || added.ID != "new" {
		t.Fatalf("POST /events = %+v, %v", added, err)
	}
	var list struct {
		Events []store.Event `json:"events"`
	}
	if err := client.Call("GET", "/events", nil, &list); err != nil || len(list.Events) != 1 || list.Events[0].ID != "new" {
		t.Fatalf("GET /events = %+v, %v; want only the new event", list, err)
	}

	var reply map[string]any
	if err := client.Call("POST", "/shutdown", nil, &reply); err != nil {
		t.Fatal(err)
	}
	wait(t, done)

	// Expired events are dropped as it runs.
	defer func(every time.Duration) { PruneEvery = every }(PruneEvery)
	PruneEvery = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	client, done = start(t, ctx, dir)
	time.Sleep(1200 * time.Millisecond)
	if err := client.Call("GET", "/events", nil, &list); err != nil || len(list.Events) != 0 {
		t.Errorf("GET /events after the new event expired = %+v, %v", list, err)
	}

	// Cancelling the context stops it too, and removes the socket.
	cancel()
	wait(t, done)
	if _, err := os.Stat(state.Socket(dir)); !os.IsNotExist(err) {
		t.Errorf("the socket is still there: %v", err)
	}
}
