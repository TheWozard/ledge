// Package shim is the native layer: AppKit windows hosting WKWebView pages, reached
// from Go through a small C API (shim.h, shim.m). It holds no app logic.
//
// AppKit runs on the main thread, so importing this package locks the main goroutine
// to it, and Run must be called from the main goroutine. Every other function is
// safe from any goroutine.
package shim

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"runtime"
	"runtime/cgo"
	"sync"
	"unsafe"
)

func init() {
	// init runs on the main goroutine, which is on the main thread at this point.
	runtime.LockOSThread()
}

// Rect is a window frame in screen points, with the origin at the bottom left.
type Rect struct{ X, Y, Width, Height float64 }

// Handlers for what the native side reports. Each is called on the main thread, so
// it must not block; hand work to a goroutine.
var (
	handlersMu       sync.Mutex
	onMessage        func(window int, message []byte)
	onScreensChanged func()
	onNavigate       func(window int, url string, clicked bool) bool
)

// OnMessage sets what's called with each JSON message a page posts.
func OnMessage(handle func(window int, message []byte)) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	onMessage = handle
}

// OnScreensChanged sets what's called when screens are added, removed or rearranged.
func OnScreensChanged(handle func()) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	onScreensChanged = handle
}

// OnNavigate sets what decides whether a window may go to url; clicked is true for
// a link the user clicked. Without one, only about:blank (a page loaded with LoadHTML)
// is allowed.
func OnNavigate(decide func(window int, url string, clicked bool) bool) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	onNavigate = decide
}

// Init sets up the app. Call it once, before creating windows.
func Init() { C.ledgeInit() }

// Run runs the main loop until Stop. Main goroutine only.
func Run() { C.ledgeRun() }

// Stop makes Run return.
func Stop() { C.ledgeStop() }

// CreateBar creates window id as the bar.
func CreateBar(id int) { C.ledgeCreateBar(C.int(id)) }

// PlaceBar puts the bar along the top or bottom of a screen and returns its frame.
func PlaceBar(id, screen int, top bool, height float64) Rect {
	r := C.ledgePlaceBar(C.int(id), C.int(screen), cBool(top), C.double(height))
	return Rect{float64(r.x), float64(r.y), float64(r.width), float64(r.height)}
}

// CreateWindow creates window id as a titled window, if it doesn't exist yet. Its
// page fills the window under a see-through title bar, so the page draws its own
// title; only the close, minimise and zoom buttons show. It can't post messages to Go,
// only move or close its own window (see SetFrameless).
func CreateWindow(id int, title string, width, height float64) {
	text := C.CString(title)
	defer C.free(unsafe.Pointer(text))
	C.ledgeCreateWindow(C.int(id), text, C.double(width), C.double(height))
}

// ShowWindow brings the window to the front and gives it focus.
func ShowWindow(id int) { C.ledgeShowWindow(C.int(id)) }

// SetFrameless takes a window's macOS frame away, or gives it back. A frameless window
// has square corners and no title bar or buttons: its page draws them, and works them
// and drags the window by posting "close", "minimise", "zoom" or "drag" to
// window.webkit.messageHandlers.frame.
func SetFrameless(id int, frameless bool) {
	C.ledgeSetFrameless(C.int(id), C.int(map[bool]int{true: 1}[frameless]))
}

// WindowVisible reports whether the window is on screen.
func WindowVisible(id int) bool { return C.ledgeWindowVisible(C.int(id)) != 0 }

// SetLevel sets which windows the window stays above: "top" for everything, the Dock
// included; "floating" for other apps' windows but not the Dock; anything else for
// none, as an ordinary window.
func SetLevel(id int, level string) {
	C.ledgeSetLevel(C.int(id), C.int(map[string]int{"floating": 1, "top": 2}[level]))
}

// SetAppearance makes the window's frame and controls (its close, minimise and zoom
// buttons, scroll bars) "light", "dark", or anything else to follow the system.
func SetAppearance(id int, appearance string) {
	C.ledgeSetAppearance(C.int(id), C.int(map[string]int{"light": 1, "dark": 2}[appearance]))
}

// SetTitle sets the window's title.
func SetTitle(id int, title string) {
	text := C.CString(title)
	defer C.free(unsafe.Pointer(text))
	C.ledgeSetTitle(C.int(id), text)
}

// LoadHTML replaces the window's page.
func LoadHTML(id int, html string) {
	text := C.CString(html)
	defer C.free(unsafe.Pointer(text))
	C.ledgeLoadHTML(C.int(id), text)
}

// Eval runs JavaScript in the window's page, without waiting for it.
func Eval(id int, js string) {
	text := C.CString(js)
	defer C.free(unsafe.Pointer(text))
	C.ledgeEval(C.int(id), text)
}

type evalResult struct {
	value json.RawMessage
	err   error
}

// EvalResult runs JavaScript in the window's page and returns its result as JSON.
// Not from the main goroutine: it waits for the main thread.
func EvalResult(id int, js string) (json.RawMessage, error) {
	done := make(chan evalResult, 1)
	handle := cgo.NewHandle(done)
	text := C.CString(js)
	defer C.free(unsafe.Pointer(text))
	C.ledgeEvalResult(C.int(id), text, C.uintptr_t(handle))
	result := <-done
	return result.value, result.err
}

func cBool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

//export goLedgeMessage
func goLedgeMessage(window C.int, message *C.char) {
	handlersMu.Lock()
	handle := onMessage
	handlersMu.Unlock()
	if handle != nil {
		handle(int(window), []byte(C.GoString(message)))
	}
}

//export goLedgeScreensChanged
func goLedgeScreensChanged() {
	handlersMu.Lock()
	handle := onScreensChanged
	handlersMu.Unlock()
	if handle != nil {
		handle()
	}
}

//export goLedgeNavigate
func goLedgeNavigate(window C.int, url *C.char, clicked C.int) C.int {
	handlersMu.Lock()
	decide := onNavigate
	handlersMu.Unlock()
	target := C.GoString(url)
	if decide == nil {
		return cBool(target == "about:blank")
	}
	return cBool(decide(int(window), target, clicked != 0))
}

//export goLedgeEvalResult
func goLedgeEvalResult(handle C.uintptr_t, value *C.char, message *C.char) {
	h := cgo.Handle(handle)
	done := h.Value().(chan evalResult)
	h.Delete()
	if message != nil {
		done <- evalResult{err: errors.New(C.GoString(message))}
		return
	}
	done <- evalResult{value: json.RawMessage(C.GoString(value))}
}
