#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "shim.h"
#include "_cgo_export.h"

// How values from a page become JSON: any value, not only objects and arrays, and
// without escaping "/" as "\/".
static const NSJSONWritingOptions jsonOptions = NSJSONWritingFragmentsAllowed | NSJSONWritingWithoutEscapingSlashes;

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
@interface LedgeWindow : NSWindow
@end
@implementation LedgeWindow
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }
@end

// Takes the first click even when ledge isn't the active app, so a click on the bar
// acts rather than only activating it.
@interface LedgeWebView : WKWebView
@end
@implementation LedgeWebView
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
@end

// Passes page messages, navigations and title changes to Go. A page with the message
// handler posts {action, id...} with window.webkit.messageHandlers.ledge.postMessage(...).
@interface LedgeDelegate : NSObject <WKScriptMessageHandler, WKNavigationDelegate>
@property int windowID;
@end
@implementation LedgeDelegate
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    // Anything JSON goes to Go, which checks it against a schema; a value that can't
    // be JSON is dropped here.
    NSData *data = [NSJSONSerialization dataWithJSONObject:message.body options:jsonOptions error:nil];
    if (data == nil) {
        return;
    }
    NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
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
    if (action.targetFrame == nil) {
        // A link to a new window (target=_blank): there are no others, so load it here.
        decisionHandler(WKNavigationActionPolicyCancel);
        [webView loadRequest:action.request];
        return;
    }
    decisionHandler(WKNavigationActionPolicyAllow);
}
- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object
                        change:(NSDictionary *)change context:(void *)context {
    NSString *title = ((WKWebView *)object).title ?: @"";
    goLedgeTitleChanged(self.windowID, (char *)title.UTF8String);
}
@end

static NSMutableDictionary<NSNumber *, NSWindow *> *windows;
static NSMutableDictionary<NSNumber *, WKWebView *> *webViews;
static NSMutableDictionary<NSNumber *, LedgeDelegate *> *delegates;

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

static WKWebView *newWebView(int window, NSRect frame, BOOL messages) {
    LedgeDelegate *delegate = [LedgeDelegate new];
    delegate.windowID = window;
    WKWebViewConfiguration *config = [WKWebViewConfiguration new];
    if (messages) {
        [config.userContentController addScriptMessageHandler:delegate name:@"ledge"];
    }
    WKWebView *webView = [[LedgeWebView alloc] initWithFrame:frame configuration:config];
    webView.navigationDelegate = delegate;
    [webView addObserver:delegate forKeyPath:@"title" options:0 context:NULL];
    webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    delegates[@(window)] = delegate;
    webViews[@(window)] = webView;
    return webView;
}

void ledgeCreateBar(int window) {
    onMainSync(^{
        NSRect frame = NSMakeRect(0, 0, 800, 110);
        NSWindow *win = [[LedgeWindow alloc] initWithContentRect:frame
                                                       styleMask:NSWindowStyleMaskBorderless
                                                         backing:NSBackingStoreBuffered
                                                           defer:NO];
        win.releasedWhenClosed = NO;
        // Left out of Cmd-` cycling; otherwise managed like any window, so it opens on
        // the current Space.
        win.collectionBehavior = NSWindowCollectionBehaviorIgnoresCycle;
        win.level = NSStatusWindowLevel;
        win.hidesOnDeactivate = NO;
        win.hasShadow = NO;
        win.contentView = newWebView(window, frame, YES);
        windows[@(window)] = win;
    });
}

void ledgeCreateWindow(int window, const char *title, double width, double height) {
    NSString *text = [NSString stringWithUTF8String:title];
    onMainSync(^{
        if (windows[@(window)] != nil) {
            return;
        }
        NSRect frame = NSMakeRect(0, 0, width, height);
        NSWindow *win = [[NSWindow alloc] initWithContentRect:frame
                                                    styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                                            | NSWindowStyleMaskResizable | NSWindowStyleMaskMiniaturizable
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO];
        win.releasedWhenClosed = NO;
        win.title = text;
        win.contentMinSize = NSMakeSize(480, 320);
        win.contentView = newWebView(window, frame, NO);
        [win center];
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

void ledgeSetTitle(int window, const char *title) {
    NSString *text = [NSString stringWithUTF8String:title];
    onMain(^{
        windows[@(window)].title = text;
    });
}

void ledgeLoadFile(int window, const char *path, const char *dir) {
    NSURL *file = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
    NSURL *access = [NSURL fileURLWithPath:[NSString stringWithUTF8String:dir] isDirectory:YES];
    onMain(^{
        [webViews[@(window)] loadFileURL:file allowingReadAccessToURL:access];
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
            NSData *data = result == nil ? nil
                : [NSJSONSerialization dataWithJSONObject:result options:jsonOptions error:nil];
            NSString *json = data == nil ? @"null" : [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
            goLedgeEvalResult(handle, (char *)json.UTF8String, NULL);
        }];
    });
}
