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

// ledgeCreateBar creates `window` as the bar: borderless and at status window level.
// It opens on the current Space and stays there, like any window.
void ledgeCreateBar(int window);
// ledgePlaceBar stretches the bar across the visible area of screen `screen` (screen
// 0 if there's no such screen) along the top or bottom edge, and returns its frame.
LedgeRect ledgePlaceBar(int window, int screen, int top, double height);

// ledgeCreateWindow creates `window` as an ordinary titled window, if it doesn't exist
// yet. Its page gets no message handler, so it can't post anything to Go.
void ledgeCreateWindow(int window, const char *title, double width, double height);
// ledgeShowWindow brings a window to the front and activates the app, so it takes focus.
void ledgeShowWindow(int window);
void ledgeSetTitle(int window, const char *title);

// Every navigation in every window asks goLedgeNavigate, which says whether to allow
// it. One that would open a new window is loaded in the same one if allowed.
void ledgeLoadHTML(int window, const char *html);
// ledgeLoadFile loads a file, letting the page read files under dir.
void ledgeLoadFile(int window, const char *path, const char *dir);
void ledgeEval(int window, const char *js);
// ledgeEvalResult runs js and reports its result, as JSON, to goLedgeEvalResult.
void ledgeEvalResult(int window, const char *js, uintptr_t handle);
