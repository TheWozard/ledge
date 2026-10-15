#pragma once

// The native shim: NSWindow + WKWebView, and nothing else. Go calls these; the shim
// calls back through the go* functions exported in shim.go. It holds no app logic.
#include <stdint.h>

typedef struct { double x, y, width, height; } LedgeRect;

// ledgeInit makes this an accessory app (no Dock icon or menu bar), gives it a menu
// of key equivalents only (copy, select all, close window), and starts reporting
// screen changes.
void ledgeInit(void);
// ledgeRun runs the main loop until ledgeStop. Main thread only.
void ledgeRun(void);
void ledgeStop(void);

// ledgeCreateBar creates `window` as the bar: borderless, at the normal window level.
// It opens on the current Space and stays there, like any window.
void ledgeCreateBar(int window);
// ledgePlaceBar stretches the bar across the visible area of screen `screen` (screen
// 0 if there's no such screen) along the top or bottom edge, and returns its frame.
LedgeRect ledgePlaceBar(int window, int screen, int top, double height);

// ledgeCreateWindow creates `window` as a titled window, if it doesn't exist yet. Its
// page fills the window under a see-through title bar that shows only the close,
// minimise and zoom buttons, so the page draws its own title. The page can't post
// anything to Go; its "frame" handler only moves, closes, minimises or zooms its window.
void ledgeCreateWindow(int window, const char *title, double width, double height);
// ledgeShowWindow brings a window to the front and activates the app, so it takes focus.
void ledgeShowWindow(int window);
void ledgeSetTitle(int window, const char *title);
// ledgeSetFrameless takes a window's macOS frame away (1) or gives it back (0). A
// frameless window has square corners and no title bar or buttons: its page draws
// them, and moves and closes the window through its "frame" message handler.
void ledgeSetFrameless(int window, int frameless);
// ledgeWindowVisible reports whether a window is on screen.
int ledgeWindowVisible(int window);
// ledgeSetLevel puts a window above everything, the Dock included (2), above other
// apps' windows but under the Dock (1), or at the normal level (0).
void ledgeSetLevel(int window, int level);
// ledgeSetAppearance makes a window's frame and controls light (1), dark (2), or follow
// the system (0).
void ledgeSetAppearance(int window, int appearance);

// Every navigation in every window asks goLedgeNavigate, which says whether to allow
// it. One that would open a new window is loaded in the same one if allowed.
void ledgeLoadHTML(int window, const char *html);
void ledgeEval(int window, const char *js);
// ledgeEvalResult runs js and reports its result, as JSON, to goLedgeEvalResult.
void ledgeEvalResult(int window, const char *js, uintptr_t handle);
