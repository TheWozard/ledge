package ui

import (
	"encoding/json"

	"github.com/TheWozard/ledge-frontend/shim"
)

// Windows is what a bar asks of its windows once it's open: loading pages in them,
// each by its window id, and running JS in them; opening and showing reports' windows
// and the menu's pop-up; the native menu; opening a link in the browser; and copying
// a Code block's text to the pasteboard. Open's are the screen's, through the shim;
// OpenIn's can be anything that shows the pages, such as a browser's frames. Opening
// and placing the bar, and the shim's handlers, are Open's alone.
//
// The bar evaluates JS in a page only once the page has posted "ready", but for
// EvalResult, which it asks of the menu's page once that has. The calls but EvalResult
// and ShowMenu, which wait for their answer, mustn't wait for the bar: it can be
// holding its lock as it makes them.
type Windows interface {
	LoadHTML(window int, html string)
	Eval(window int, js string)
	EvalResult(window int, js string) (json.RawMessage, error)

	CreateWindow(window int, title string, width, height float64)
	SetTitle(window int, title string)
	SetAppearance(window int, appearance string)
	SetFrameless(window int, frameless bool)
	ShowWindow(window int)

	CreatePopup(window int)
	ShowPopup(window int, width, height float64)
	HidePopup(window int)
	ShowMenu(items []shim.MenuItem) shim.Chosen

	OpenURL(url string)
	Copy(text string)
}

// shimNative is the screen's Windows, through the shim.
type shimNative struct{}

func (shimNative) LoadHTML(window int, html string) { shim.LoadHTML(window, html) }
func (shimNative) Eval(window int, js string)       { shim.Eval(window, js) }
func (shimNative) EvalResult(window int, js string) (json.RawMessage, error) {
	return shim.EvalResult(window, js)
}

func (shimNative) CreateWindow(window int, title string, width, height float64) {
	shim.CreateWindow(window, title, width, height)
}
func (shimNative) SetTitle(window int, title string) { shim.SetTitle(window, title) }
func (shimNative) SetAppearance(window int, appearance string) {
	shim.SetAppearance(window, appearance)
}
func (shimNative) SetFrameless(window int, frameless bool) { shim.SetFrameless(window, frameless) }
func (shimNative) ShowWindow(window int)                   { shim.ShowWindow(window) }

func (shimNative) CreatePopup(window int) { shim.CreatePopup(window) }
func (shimNative) ShowPopup(window int, width, height float64) {
	shim.ShowPopup(window, width, height)
}
func (shimNative) HidePopup(window int)                       { shim.HidePopup(window) }
func (shimNative) ShowMenu(items []shim.MenuItem) shim.Chosen { return shim.ShowMenu(items) }

func (shimNative) OpenURL(url string) { shim.OpenURL(url) }
func (shimNative) Copy(text string)   { shim.Copy(text) }
