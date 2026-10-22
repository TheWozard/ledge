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
	"strings"
	"sync"
	"unsafe"
)

func init() {
	// init runs on the main goroutine, which is on the main thread at this point.
	runtime.LockOSThread()
}

// Rect is a window frame in screen points, with the origin at the bottom left.
type Rect struct{ X, Y, Width, Height float64 }

// handlers is what's called with what the native side reports, each set by its On
// function. Each is called on the main thread, so it must not block; hand work to a
// goroutine.
var handlers struct {
	sync.Mutex
	message        func(window int, message []byte)
	screensChanged func()
	navigate       func(window int, url string, clicked bool) bool
	windowClosed   func(window int)
	popupDismissed func(window int)
}

// setHandler sets one of handlers.
func setHandler[T any](handler *T, to T) {
	handlers.Lock()
	defer handlers.Unlock()
	*handler = to
}

// getHandler is one of handlers, as it's set now.
func getHandler[T any](handler *T) T {
	handlers.Lock()
	defer handlers.Unlock()
	return *handler
}

// OnMessage sets what's called with each JSON message a page posts.
func OnMessage(handle func(window int, message []byte)) { setHandler(&handlers.message, handle) }

// OnScreensChanged sets what's called when screens are added, removed or rearranged.
func OnScreensChanged(handle func()) { setHandler(&handlers.screensChanged, handle) }

// OnNavigate sets what decides whether a window may go to url; clicked is true for
// a link the user clicked. Without one, only about:blank (a page loaded with LoadHTML)
// is allowed.
func OnNavigate(decide func(window int, url string, clicked bool) bool) {
	setHandler(&handlers.navigate, decide)
}

// OnWindowClosed sets what's called when a titled window closes. It's then gone: the
// shim forgets it, and CreateWindow makes a new one.
func OnWindowClosed(handle func(window int)) { setHandler(&handlers.windowClosed, handle) }

// OnPopupDismissed sets what's called when a pop-up is dismissed by a click anywhere
// but on it. It's hidden by then; it isn't called for HidePopup.
func OnPopupDismissed(handle func(window int)) { setHandler(&handlers.popupDismissed, handle) }

// Init sets up the app. Call it once, before creating windows.
func Init() { C.ledgeInit() }

// Run runs the main loop until Stop. Main goroutine only.
func Run() { C.ledgeRun() }

// Stop makes Run return.
func Stop() { C.ledgeStop() }

// CreateBar creates window id as the bar, if it doesn't exist yet.
func CreateBar(id int) { C.ledgeCreateBar(C.int(id)) }

// PlaceBar puts the bar along the top or bottom of a screen and returns its frame.
func PlaceBar(id, screen int, top bool, height float64) Rect {
	return goRect(C.ledgePlaceBar(C.int(id), C.int(screen), cBool(top), C.double(height)))
}

// CreatePopup creates window id as a pop-up, if it doesn't exist yet: borderless and
// see-through where its page draws nothing, above other windows, and taking keys
// without taking focus from the app that has it. Its page can post messages to Go,
// as the bar's can. It's hidden until ShowPopup.
func CreatePopup(id int) { C.ledgeCreatePopup(C.int(id)) }

// ShowPopup shows the pop-up width by height at the mouse, down and to the right of
// it, or up or left to stay on its screen, and gives it the keys. A click anywhere
// but on it dismisses it (see OnPopupDismissed).
func ShowPopup(id int, width, height float64) {
	C.ledgeShowPopup(C.int(id), C.double(width), C.double(height))
}

// HidePopup hides the pop-up, without it being dismissed.
func HidePopup(id int) { C.ledgeHidePopup(C.int(id)) }

// CreateWindow creates window id as a titled window, if it doesn't exist yet, down
// and to the right of the last one. Closing it destroys it (see OnWindowClosed). Its
// page fills the window under a see-through title bar, so the page draws its own
// title; only the close, minimise and zoom buttons show. It can't post messages to Go,
// only move or close its own window (see SetFrameless).
func CreateWindow(id int, title string, width, height float64) {
	text := cString(title)
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
	C.ledgeSetFrameless(C.int(id), cBool(frameless))
}

// WindowVisible reports whether the window is on screen.
func WindowVisible(id int) bool { return C.ledgeWindowVisible(C.int(id)) != 0 }

// WindowFrame is the window's frame, or the zero Rect if there's no such window.
func WindowFrame(id int) Rect { return goRect(C.ledgeWindowFrame(C.int(id))) }

// SetLevel sets which windows the window stays above: "top" for everything, the Dock
// included; "floating" for other apps' windows but not the Dock; anything else for
// none, as an ordinary window.
func SetLevel(id int, level string) { C.ledgeSetLevel(C.int(id), levels[level]) }

// levels is each level SetLevel takes by name; any other is LedgeLevelNormal.
var levels = map[string]C.LedgeLevel{"floating": C.LedgeLevelFloating, "top": C.LedgeLevelTop}

// SetAppearance makes the window's frame and controls (its close, minimise and zoom
// buttons, scroll bars) "light", "dark", or anything else to follow the system.
func SetAppearance(id int, appearance string) {
	C.ledgeSetAppearance(C.int(id), appearances[appearance])
}

// appearances is each appearance SetAppearance takes by name; any other is
// LedgeAppearanceSystem.
var appearances = map[string]C.LedgeAppearance{"light": C.LedgeAppearanceLight, "dark": C.LedgeAppearanceDark}

// SetTitle sets the window's title.
func SetTitle(id int, title string) {
	text := cString(title)
	defer C.free(unsafe.Pointer(text))
	C.ledgeSetTitle(C.int(id), text)
}

// LoadHTML replaces the window's page.
func LoadHTML(id int, html string) {
	text := cString(html)
	defer C.free(unsafe.Pointer(text))
	C.ledgeLoadHTML(C.int(id), text)
}

// Eval runs JavaScript in the window's page, without waiting for it.
func Eval(id int, js string) {
	text := cString(js)
	defer C.free(unsafe.Pointer(text))
	C.ledgeEval(C.int(id), text)
}

// OpenURL opens url in its default app, such as the browser for an http(s) one.
func OpenURL(url string) {
	text := cString(url)
	defer C.free(unsafe.Pointer(text))
	C.ledgeOpenURL(text)
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
	text := cString(js)
	defer C.free(unsafe.Pointer(text))
	C.ledgeEvalResult(C.int(id), text, C.uintptr_t(handle))
	result := <-done
	return result.value, result.err
}

// MenuItem is an item in a menu ShowMenu pops up.
type MenuItem struct {
	Title string
	// Color is the colour of a dot before the title: green, yellow, red, blue or gray,
	// as the system's, or #rrggbb; "" for none.
	Color   string
	Enabled bool // whether it can be chosen
}

// Modifiers is the modifier keys held as a menu item was chosen.
type Modifiers struct {
	Alt, Cmd, Shift, Ctrl bool
}

// Chosen is what was chosen from a menu: the item's index, or -1 for none, and the
// modifier keys held as it was.
type Chosen struct {
	Index     int
	Modifiers Modifiers
}

// ShowMenu pops up a menu of items at the mouse, and returns what was chosen from it.
// Not from the main goroutine: it waits for the menu to close. While it's open,
// everything else still runs.
func ShowMenu(items []MenuItem) Chosen {
	if len(items) == 0 {
		return Chosen{Index: -1}
	}
	array := unsafe.Slice((*C.LedgeMenuItem)(C.malloc(C.size_t(len(items))*C.size_t(unsafe.Sizeof(C.LedgeMenuItem{})))), len(items))
	defer C.free(unsafe.Pointer(&array[0]))
	for i, item := range items {
		title, color := cString(item.Title), cString(item.Color)
		defer C.free(unsafe.Pointer(title))
		defer C.free(unsafe.Pointer(color))
		array[i] = C.LedgeMenuItem{title: title, color: color, enabled: cBool(item.Enabled)}
	}
	done := make(chan Chosen, 1)
	C.ledgeShowMenu(&array[0], C.int(len(items)), C.uintptr_t(cgo.NewHandle(done)))
	return <-done
}

// cString is s as a C string, made safe by cText. Free it with C.free.
func cString(s string) *C.char { return C.CString(cText(s)) }

// cText is s as the native side can take it whole: valid UTF-8, which NSString needs
// (it's nil otherwise), and without NULs, which would end it early.
func cText(s string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "\uFFFD")
}

// goRect is a frame from the native side as a Rect.
func goRect(r C.LedgeRect) Rect {
	return Rect{float64(r.x), float64(r.y), float64(r.width), float64(r.height)}
}

func cBool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

//export goLedgeMessage
func goLedgeMessage(window C.int, message *C.char) {
	if handle := getHandler(&handlers.message); handle != nil {
		handle(int(window), []byte(C.GoString(message)))
	}
}

//export goLedgeScreensChanged
func goLedgeScreensChanged() {
	if handle := getHandler(&handlers.screensChanged); handle != nil {
		handle()
	}
}

//export goLedgeWindowClosed
func goLedgeWindowClosed(window C.int) {
	if handle := getHandler(&handlers.windowClosed); handle != nil {
		handle(int(window))
	}
}

//export goLedgeNavigate
func goLedgeNavigate(window C.int, url *C.char, clicked C.int) C.int {
	decide := getHandler(&handlers.navigate)
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

//export goLedgeMenuChosen
func goLedgeMenuChosen(handle C.uintptr_t, index C.int, modifiers C.int) {
	h := cgo.Handle(handle)
	done := h.Value().(chan Chosen)
	h.Delete()
	done <- Chosen{Index: int(index), Modifiers: Modifiers{
		Alt:   modifiers&C.LedgeAlt != 0,
		Cmd:   modifiers&C.LedgeCmd != 0,
		Shift: modifiers&C.LedgeShift != 0,
		Ctrl:  modifiers&C.LedgeCtrl != 0,
	}}
}

//export goLedgePopupDismissed
func goLedgePopupDismissed(window C.int) {
	if handle := getHandler(&handlers.popupDismissed); handle != nil {
		handle(int(window))
	}
}
