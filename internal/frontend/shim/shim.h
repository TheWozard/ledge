#pragma once

// The native shim: NSWindow + WKWebView, and nothing else. Go calls these; the shim
// calls back through the go* functions exported in shim.go. It holds no app logic.
#include <stdint.h>

typedef struct { double x, y, width, height; } LedgeRect;

// Which windows a window stays above: none (an ordinary window), other apps' windows
// but not the Dock, or everything, the Dock included.
typedef enum { LedgeLevelNormal, LedgeLevelFloating, LedgeLevelTop } LedgeLevel;
// How a window's frame and controls look: as the system does, light or dark.
typedef enum { LedgeAppearanceSystem, LedgeAppearanceLight, LedgeAppearanceDark } LedgeAppearance;

// ledgeInit makes this an accessory app (no Dock icon or menu bar), gives it a menu
// of key equivalents only (copy, select all, close window), and starts reporting
// screen changes.
void ledgeInit(void);
// ledgeRun runs the main loop until ledgeStop. Main thread only.
void ledgeRun(void);
void ledgeStop(void);

// ledgeCreateBar creates `window` as the bar, if it doesn't exist yet: borderless, at the normal window level.
// It opens on the current Space and stays there, like any window.
void ledgeCreateBar(int window);
// ledgePlaceBar stretches the bar across the visible area of screen `screen` (screen
// 0 if there's no such screen) along the top or bottom edge, and returns its frame.
LedgeRect ledgePlaceBar(int window, int screen, int top, double height);

// ledgeCreateWindow creates `window` as a titled window, if it doesn't exist yet,
// down and to the right of the last one. Its page fills the window under a
// see-through title bar that shows only the close, minimise and zoom buttons, so the
// page draws its own title. The page can't post anything to Go; its "frame" handler
// only moves, closes, minimises or zooms its window. Closing it destroys it, and is
// reported to goLedgeWindowClosed; `window` can then be created again.
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
// ledgeWindowFrame is a window's frame, or all zeros if there's no such window.
LedgeRect ledgeWindowFrame(int window);
// ledgeSetLevel sets which windows a window stays above.
void ledgeSetLevel(int window, LedgeLevel level);
// ledgeSetAppearance sets how a window's frame and controls look.
void ledgeSetAppearance(int window, LedgeAppearance appearance);

// Every navigation in every window asks goLedgeNavigate, which says whether to allow
// it. One that would open a new window goes nowhere.
void ledgeLoadHTML(int window, const char *html);
void ledgeEval(int window, const char *js);
// ledgeEvalResult runs js and reports its result, as JSON, to goLedgeEvalResult.
void ledgeEvalResult(int window, const char *js, uintptr_t handle);
