#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "shim.h"
#include "_cgo_export.h"

// How values from a page become JSON: any value, not only objects and arrays, and
// without escaping "/" as "\/".
static const NSJSONWritingOptions jsonOptions = NSJSONWritingFragmentsAllowed | NSJSONWritingWithoutEscapingSlashes;

// jsonString is value as JSON, or nil if it can't be: NSJSONSerialization throws,
// rather than failing, on a value such as a Date or NaN.
static NSString *jsonString(id value) {
    NSData *data = nil;
    @try {
        data = [NSJSONSerialization dataWithJSONObject:value options:jsonOptions error:nil];
    } @catch (NSException *e) {
        return nil;
    }
    return data == nil ? nil : [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
}

// Runs block on the main thread, waiting for it. Calls already on the main thread
// run straight away, so a callback into Go can call back into the shim.
static void onMainSync(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

// Runs block on the main thread without waiting, so a caller never blocks on AppKit.
static void onMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

// A borderless window can't become key by default, which stops the page taking clicks.
// Nor can it close with performClose: (Cmd-W, or a page's close button), which acts
// as a click on the close button it hasn't got, so it just closes.
@interface LedgeWindow : NSWindow
@end
@implementation LedgeWindow
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }
- (void)performClose:(id)sender {
    if ([self standardWindowButton:NSWindowCloseButton] == nil) {
        [self close];
    } else {
        [super performClose:sender];
    }
}
@end

// The bar never closes: Cmd-W while it's key would otherwise close it for good.
@interface LedgeBarWindow : LedgeWindow
@end
@implementation LedgeBarWindow
- (void)performClose:(id)sender {}
@end

// Takes the first click even when ledge isn't the active app, so a click on the bar
// acts rather than only activating it. Keeps the last mouse-down, which a window drag
// the page asks for starts from.
@interface LedgeWebView : WKWebView
@property(strong) NSEvent *lastMouseDown;
@end
@implementation LedgeWebView
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
- (void)mouseDown:(NSEvent *)event {
    self.lastMouseDown = event;
    [super mouseDown:event];
}
@end

// Lets a page move and close its own window, as the title bar it draws would. It posts
// one of four strings with window.webkit.messageHandlers.frame.postMessage(...):
// "drag" on a mouse-down in its title bar, and "close", "minimise" or "zoom" from its
// own buttons. Anything else is dropped, and none of it reaches Go.
@interface LedgeFrameHandler : NSObject <WKScriptMessageHandler>
@end
@implementation LedgeFrameHandler
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    LedgeWebView *webView = (LedgeWebView *)message.webView;
    NSWindow *win = webView.window;
    if (win == nil || ![message.body isKindOfClass:[NSString class]]) {
        return;
    }
    NSString *action = message.body;
    if ([action isEqualToString:@"drag"]) {
        NSEvent *down = webView.lastMouseDown;
        // The page posts as the button goes down, so it's normally still held; a
        // click that's already over doesn't start a drag.
        if (down == nil || (NSEvent.pressedMouseButtons & 1) == 0) {
            return;
        }
        if (down.clickCount == 2) {
            // A double-click on a title bar does what System Settings says.
            NSString *pref = [[NSUserDefaults standardUserDefaults] stringForKey:@"AppleActionOnDoubleClick"];
            if ([pref isEqualToString:@"Minimize"]) {
                [win miniaturize:nil];
            } else if (![pref isEqualToString:@"None"]) {
                [win zoom:nil];
            }
            return;
        }
        [win performWindowDragWithEvent:down];
    } else if ([action isEqualToString:@"close"]) {
        [win performClose:nil];
    } else if ([action isEqualToString:@"minimise"]) {
        [win miniaturize:nil];
    } else if ([action isEqualToString:@"zoom"]) {
        [win zoom:nil];
    }
}
@end

// The page each window was last given, to load again if its web content process dies.
static NSMutableDictionary<NSNumber *, NSString *> *pages;

@class LedgeDelegate;
static NSMutableDictionary<NSNumber *, NSWindow *> *windows;
static NSMutableDictionary<NSNumber *, WKWebView *> *webViews;
static NSMutableDictionary<NSNumber *, LedgeDelegate *> *delegates;

// Passes page messages, navigations and a titled window closing to Go. A page with
// the message handler posts {action, id...} with
// window.webkit.messageHandlers.ledge.postMessage(...).
@interface LedgeDelegate : NSObject <WKScriptMessageHandler, WKNavigationDelegate, NSWindowDelegate>
@property int windowID;
@end
@implementation LedgeDelegate
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    // Anything JSON goes to Go, which checks it against a schema; a value that can't
    // be JSON is dropped here.
    NSString *json = jsonString(message.body);
    if (json == nil) {
        return;
    }
    goLedgeMessage(self.windowID, (char *)json.UTF8String);
}
- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    NSString *url = action.request.URL.absoluteString ?: @"";
    int clicked = action.navigationType == WKNavigationTypeLinkActivated;
    if (!goLedgeNavigate(self.windowID, (char *)url.UTF8String, clicked)) {
        decisionHandler(WKNavigationActionPolicyCancel);
        return;
    }
    // One for a new window (target=_blank) does nothing even if allowed: there's no
    // WKUIDelegate to make one.
    decisionHandler(WKNavigationActionPolicyAllow);
}
// The page's process was killed (memory pressure, a WebKit crash), leaving the window
// blank: load its page again. The bar's page posts "ready" again and is redrawn.
- (void)webViewWebContentProcessDidTerminate:(WKWebView *)webView {
    NSString *page = pages[@(self.windowID)];
    if (page != nil) {
        [webView loadHTMLString:page baseURL:nil];
    }
}
// A titled window closing is gone for good: it's let go once it has closed, and Go is
// told, so the id isn't shown again.
- (void)windowWillClose:(NSNotification *)note {
    int window = self.windowID;
    dispatch_async(dispatch_get_main_queue(), ^{
        [windows removeObjectForKey:@(window)];
        [webViews removeObjectForKey:@(window)];
        [delegates removeObjectForKey:@(window)];
        [pages removeObjectForKey:@(window)];
    });
    goLedgeWindowClosed(window);
}
@end

// keyMenu is a menu that's never shown (an accessory app has no menu bar) but gives
// the usual key equivalents: Cmd-C and Cmd-A in a page, Cmd-W to close a window.
static NSMenu *keyMenu(void) {
    NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Edit"];
    [edit addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
    [edit addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
    [edit addItemWithTitle:@"Close Window" action:@selector(performClose:) keyEquivalent:@"w"];
    NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
    [menu addItemWithTitle:@"Edit" action:nil keyEquivalent:@""].submenu = edit;
    return menu;
}

void ledgeInit(void) {
    onMainSync(^{
        windows = [NSMutableDictionary dictionary];
        webViews = [NSMutableDictionary dictionary];
        delegates = [NSMutableDictionary dictionary];
        pages = [NSMutableDictionary dictionary];
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        NSApp.mainMenu = keyMenu();
        [[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidChangeScreenParametersNotification
                                                          object:nil
                                                           queue:[NSOperationQueue mainQueue]
                                                      usingBlock:^(NSNotification *note) { goLedgeScreensChanged(); }];
    });
}

void ledgeRun(void) {
    [NSApp run];
}

void ledgeStop(void) {
    onMain(^{
        [NSApp stop:nil];
        // stop: only takes effect after the next event, so post one.
        NSEvent *wake = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint
                                      modifierFlags:0 timestamp:0 windowNumber:0 context:nil
                                            subtype:0 data1:0 data2:0];
        [NSApp postEvent:wake atStart:YES];
    });
}

// newWebView makes a window's web view. A page in the bar can post messages to Go; a
// page in any other window can only ask for its own window to move or close.
static WKWebView *newWebView(int window, NSRect frame, BOOL messages) {
    LedgeDelegate *delegate = [LedgeDelegate new];
    delegate.windowID = window;
    WKWebViewConfiguration *config = [WKWebViewConfiguration new];
    if (messages) {
        [config.userContentController addScriptMessageHandler:delegate name:@"ledge"];
    } else {
        [config.userContentController addScriptMessageHandler:[LedgeFrameHandler new] name:@"frame"];
    }
    WKWebView *webView = [[LedgeWebView alloc] initWithFrame:frame configuration:config];
    webView.navigationDelegate = delegate;
    webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    delegates[@(window)] = delegate;
    webViews[@(window)] = webView;
    return webView;
}

void ledgeCreateBar(int window) {
    onMainSync(^{
        if (windows[@(window)] != nil) {
            return;
        }
        NSRect frame = NSMakeRect(0, 0, 800, 110);
        NSWindow *win = [[LedgeBarWindow alloc] initWithContentRect:frame
                                                          styleMask:NSWindowStyleMaskBorderless
                                                            backing:NSBackingStoreBuffered
                                                              defer:NO];
        win.releasedWhenClosed = NO;
        // Left out of Cmd-` cycling; otherwise managed like any window, so it opens on
        // the current Space. It starts at the normal level; see ledgeSetLevel.
        win.collectionBehavior = NSWindowCollectionBehaviorIgnoresCycle;
        win.hidesOnDeactivate = NO;
        win.hasShadow = NO;
        win.contentView = newWebView(window, frame, YES);
        windows[@(window)] = win;
    });
}

// Where the next titled window goes: each opens down and to the right of the last.
static NSPoint cascade;

// A pop-up takes keys, for its page's own keyboard handling, without making the app
// active: whatever app was in front stays in front.
@interface LedgePopup : NSPanel
@end
@implementation LedgePopup
- (BOOL)canBecomeKeyWindow { return YES; }
@end

// The event monitors that dismiss the pop-up that's showing, on a click anywhere
// but on it: in another app, or in another of ledge's windows.
static id popupGlobalMonitor, popupLocalMonitor;

static void stopWatchingPopup(void) {
    if (popupGlobalMonitor != nil) {
        [NSEvent removeMonitor:popupGlobalMonitor];
        popupGlobalMonitor = nil;
    }
    if (popupLocalMonitor != nil) {
        [NSEvent removeMonitor:popupLocalMonitor];
        popupLocalMonitor = nil;
    }
}

// dismissPopup hides the pop-up and tells Go, which hasn't asked for it. Main thread.
static void dismissPopup(int window) {
    NSWindow *panel = windows[@(window)];
    if (panel == nil || !panel.isVisible) {
        return;
    }
    stopWatchingPopup();
    [panel orderOut:nil];
    goLedgePopupDismissed(window);
}

void ledgeCreatePopup(int window) {
    onMainSync(^{
        if (windows[@(window)] != nil) {
            return;
        }
        // Big enough to lay its page out in before it's measured and sized.
        NSRect frame = NSMakeRect(0, 0, 400, 600);
        LedgePopup *panel = [[LedgePopup alloc] initWithContentRect:frame
                                                          styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
                                                            backing:NSBackingStoreBuffered
                                                              defer:NO];
        panel.releasedWhenClosed = NO;
        panel.level = NSPopUpMenuWindowLevel;
        panel.floatingPanel = YES;
        panel.hidesOnDeactivate = NO;
        panel.hasShadow = YES;
        panel.opaque = NO;
        panel.backgroundColor = NSColor.clearColor;
        WKWebView *webView = newWebView(window, frame, YES);
        // See-through where the page draws nothing, so its theme's menu is the shape.
        [webView setValue:@NO forKey:@"drawsBackground"];
        panel.contentView = webView;
        windows[@(window)] = panel;
    });
}

void ledgeShowPopup(int window, double width, double height) {
    onMain(^{
        NSWindow *panel = windows[@(window)];
        if (panel == nil) {
            return;
        }
        NSPoint at = [NSEvent mouseLocation];
        NSScreen *screen = NSScreen.mainScreen;
        for (NSScreen *s in NSScreen.screens) {
            if (NSPointInRect(at, s.frame)) {
                screen = s;
            }
        }
        // Down and to the right of the mouse, as a menu opens; up, or left, if that
        // would leave its screen, as from a bar along the bottom.
        NSRect area = screen.visibleFrame;
        CGFloat x = at.x, y = at.y - height;
        if (y < NSMinY(area)) {
            y = MIN(at.y, NSMaxY(area) - height);
        }
        if (x + width > NSMaxX(area)) {
            x = at.x - width;
        }
        x = MAX(x, NSMinX(area));
        y = MAX(y, NSMinY(area));
        [panel setFrame:NSMakeRect(x, y, width, height) display:YES];
        [panel orderFrontRegardless];
        [panel makeKeyWindow];
        [panel invalidateShadow];

        stopWatchingPopup();
        NSEventMask clicks = NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown | NSEventMaskOtherMouseDown;
        popupGlobalMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:clicks handler:^(NSEvent *event) {
            dismissPopup(window);
        }];
        popupLocalMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:clicks handler:^NSEvent *(NSEvent *event) {
            if (event.window != panel) {
                dismissPopup(window);
            }
            return event;
        }];
    });
}

void ledgeHidePopup(int window) {
    onMain(^{
        stopWatchingPopup();
        [windows[@(window)] orderOut:nil];
    });
}

// The report window's style: titled, with the page under a see-through title bar. A
// frameless one drops the title bar, and with it the rounded corners and buttons.
static const NSWindowStyleMask titledMask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
    | NSWindowStyleMaskResizable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskFullSizeContentView;

void ledgeCreateWindow(int window, const char *title, double width, double height) {
    NSString *text = [NSString stringWithUTF8String:title];
    onMainSync(^{
        if (windows[@(window)] != nil) {
            return;
        }
        NSRect frame = NSMakeRect(0, 0, width, height);
        // A LedgeWindow, so it can still take keys with ledgeSetFrameless.
        NSWindow *win = [[LedgeWindow alloc] initWithContentRect:frame
                                                       styleMask:titledMask
                                                         backing:NSBackingStoreBuffered
                                                           defer:NO];
        win.releasedWhenClosed = NO;
        // The page fills the whole window, title bar included: the title bar is
        // see-through and its title hidden, leaving only the close, minimise and zoom
        // buttons over the page, which draws its own title. The title is still set,
        // for Mission Control and the Window menu of other apps.
        win.titlebarAppearsTransparent = YES;
        win.titleVisibility = NSWindowTitleHidden;
        win.title = text;
        win.contentMinSize = NSMakeSize(480, 320);
        win.contentView = newWebView(window, frame, NO);
        win.delegate = delegates[@(window)];
        [win center];
        cascade = [win cascadeTopLeftFromPoint:cascade];
        windows[@(window)] = win;
    });
}

void ledgeShowWindow(int window) {
    onMain(^{
        [windows[@(window)] makeKeyAndOrderFront:nil];
        // An accessory app isn't brought forward by showing a window; without this the
        // window can open behind whatever you were using.
        if (@available(macOS 14.0, *)) {
            [NSApp activate];
        } else {
            [NSApp activateIgnoringOtherApps:YES];
        }
    });
}

void ledgeSetFrameless(int window, int frameless) {
    onMainSync(^{
        NSWindow *win = windows[@(window)];
        NSWindowStyleMask mask = frameless ? titledMask & ~NSWindowStyleMaskTitled : titledMask;
        if (win == nil || win.styleMask == mask) {
            return;
        }
        win.styleMask = mask;
        win.hasShadow = YES;
        if (!frameless) {
            win.titlebarAppearsTransparent = YES;
            win.titleVisibility = NSWindowTitleHidden;
        }
    });
}

int ledgeWindowVisible(int window) {
    __block int visible = 0;
    onMainSync(^{
        visible = windows[@(window)].isVisible;
    });
    return visible;
}

LedgeRect ledgeWindowFrame(int window) {
    __block NSRect frame = NSZeroRect;
    onMainSync(^{
        NSWindow *win = windows[@(window)];
        if (win != nil) {
            frame = win.frame;
        }
    });
    return (LedgeRect){frame.origin.x, frame.origin.y, frame.size.width, frame.size.height};
}

void ledgeSetLevel(int window, LedgeLevel level) {
    NSWindowLevel to = NSNormalWindowLevel;
    switch (level) {
    case LedgeLevelTop: to = NSStatusWindowLevel; break;
    case LedgeLevelFloating: to = NSFloatingWindowLevel; break;
    case LedgeLevelNormal: break;
    }
    onMain(^{
        windows[@(window)].level = to;
    });
}

void ledgeSetAppearance(int window, LedgeAppearance appearance) {
    NSAppearanceName name = nil;
    switch (appearance) {
    case LedgeAppearanceLight: name = NSAppearanceNameAqua; break;
    case LedgeAppearanceDark: name = NSAppearanceNameDarkAqua; break;
    case LedgeAppearanceSystem: break;
    }
    onMain(^{
        windows[@(window)].appearance = name ? [NSAppearance appearanceNamed:name] : nil;
    });
}

void ledgeSetTitle(int window, const char *title) {
    NSString *text = [NSString stringWithUTF8String:title];
    onMain(^{
        windows[@(window)].title = text;
    });
}

LedgeRect ledgePlaceBar(int window, int screen, int top, double height) {
    __block LedgeRect placed = {0, 0, 0, 0};
    onMainSync(^{
        NSWindow *win = windows[@(window)];
        NSArray<NSScreen *> *screens = [NSScreen screens];
        if (win == nil || screens.count == 0) {
            return;
        }
        NSScreen *target = screen >= 0 && (NSUInteger)screen < screens.count ? screens[screen] : screens[0];
        NSRect area = target.visibleFrame;
        CGFloat h = MIN(height, area.size.height);
        CGFloat y = top ? NSMaxY(area) - h : area.origin.y;
        NSRect frame = NSMakeRect(area.origin.x, y, area.size.width, h);
        [win setFrame:frame display:YES];
        [win orderFrontRegardless];
        placed = (LedgeRect){frame.origin.x, frame.origin.y, frame.size.width, frame.size.height};
    });
    return placed;
}

void ledgeLoadHTML(int window, const char *html) {
    NSString *text = [NSString stringWithUTF8String:html];
    onMain(^{
        pages[@(window)] = text;
        [webViews[@(window)] loadHTMLString:text baseURL:nil];
    });
}

void ledgeEval(int window, const char *js) {
    NSString *text = [NSString stringWithUTF8String:js];
    onMain(^{
        [webViews[@(window)] evaluateJavaScript:text completionHandler:nil];
    });
}

void ledgeEvalResult(int window, const char *js, uintptr_t handle) {
    NSString *text = [NSString stringWithUTF8String:js];
    onMain(^{
        WKWebView *webView = webViews[@(window)];
        if (webView == nil) {
            goLedgeEvalResult(handle, NULL, "no such window");
            return;
        }
        [webView evaluateJavaScript:text completionHandler:^(id result, NSError *error) {
            if (error != nil) {
                goLedgeEvalResult(handle, NULL, (char *)error.localizedDescription.UTF8String);
                return;
            }
            if (result == nil) {
                goLedgeEvalResult(handle, "null", NULL);
                return;
            }
            NSString *json = jsonString(result);
            if (json == nil) {
                goLedgeEvalResult(handle, NULL, "the result isn't JSON");
                return;
            }
            goLedgeEvalResult(handle, (char *)json.UTF8String, NULL);
        }];
    });
}

// dotColor is the colour of a menu item's dot: a named colour as the system's, #rrggbb,
// or nil for none.
static NSColor *dotColor(NSString *name) {
    NSDictionary<NSString *, NSColor *> *named = @{
        @"green" : NSColor.systemGreenColor, @"yellow" : NSColor.systemYellowColor,
        @"red" : NSColor.systemRedColor, @"blue" : NSColor.systemBlueColor, @"gray" : NSColor.systemGrayColor,
    };
    if (named[name] != nil) {
        return named[name];
    }
    unsigned int rgb = 0;
    NSScanner *scanner = name.length == 7 && [name hasPrefix:@"#"] ? [NSScanner scannerWithString:[name substringFromIndex:1]] : nil;
    if (scanner == nil || ![scanner scanHexInt:&rgb] || !scanner.isAtEnd) {
        return nil;
    }
    return [NSColor colorWithSRGBRed:((rgb >> 16) & 0xff) / 255.0 green:((rgb >> 8) & 0xff) / 255.0 blue:(rgb & 0xff) / 255.0 alpha:1];
}

// dot is a small round image in color, for before a menu item's title.
static NSImage *dot(NSColor *color) {
    return [NSImage imageWithSize:NSMakeSize(10, 10) flipped:NO drawingHandler:^BOOL(NSRect rect) {
        [color setFill];
        [[NSBezierPath bezierPathWithOvalInRect:NSInsetRect(rect, 1, 1)] fill];
        return YES;
    }];
}

// Reports what was chosen from a menu ledgeShowMenu popped up, once: the item's
// index when its action is sent, or -1 if the menu closed without one.
@interface LedgeMenuTarget : NSObject
@property uintptr_t handle;
@property BOOL reported;
@end
@implementation LedgeMenuTarget
- (void)report:(int)index {
    if (!self.reported) {
        self.reported = YES;
        goLedgeMenuChosen(self.handle, index);
    }
}
- (void)choose:(NSMenuItem *)item {
    [self report:(int)item.tag];
}
@end

void ledgeShowMenu(const LedgeMenuItem *items, int count, uintptr_t handle) {
    NSMutableArray<NSMenuItem *> *made = [NSMutableArray array];
    for (int i = 0; i < count; i++) {
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:[NSString stringWithUTF8String:items[i].title]
                                                      action:@selector(choose:)
                                               keyEquivalent:@""];
        item.tag = i;
        item.enabled = items[i].enabled != 0;
        NSColor *color = dotColor([NSString stringWithUTF8String:items[i].color]);
        if (color != nil) {
            item.image = dot(color);
        }
        [made addObject:item];
    }
    // From the run loop rather than the main queue: the menu runs its own loop until
    // it closes, and the queue would wait for it.
    CFRunLoopPerformBlock(CFRunLoopGetMain(), kCFRunLoopCommonModes, ^{
        LedgeMenuTarget *target = [LedgeMenuTarget new];
        target.handle = handle;
        NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
        menu.autoenablesItems = NO;
        for (NSMenuItem *item in made) {
            item.target = target;
            [menu addItem:item];
        }
        // An item's action is sent as the menu closes; without one, nothing was chosen.
        // Should a chosen item's action not come, nothing is reported as chosen a
        // moment later, so Go isn't left waiting.
        if (![menu popUpMenuPositioningItem:nil atLocation:[NSEvent mouseLocation] inView:nil]) {
            [target report:-1];
            return;
        }
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, NSEC_PER_SEC / 4), dispatch_get_main_queue(), ^{
            [target report:-1];
        });
    });
    CFRunLoopWakeUp(CFRunLoopGetMain());
}
