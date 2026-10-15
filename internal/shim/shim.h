#pragma once

// The native shim: NSWindow + WKWebView, and nothing else. Go calls these; the shim
// calls back through the go* functions exported in shim.go. It holds no app logic.
#include <stdint.h>

typedef struct { double x, y, width, height; } LedgeRect;

// ledgeInit makes this an accessory app (no Dock icon or menu bar) and starts
// reporting screen changes.
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

void ledgeLoadHTML(int window, const char *html);
void ledgeEval(int window, const char *js);
// ledgeEvalResult runs js and reports its result, as JSON, to goLedgeEvalResult.
void ledgeEvalResult(int window, const char *js, uintptr_t handle);
