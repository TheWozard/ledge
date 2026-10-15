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

// Passes page messages to Go and keeps the page from navigating anywhere. A page
// posts {action, id...} with window.webkit.messageHandlers.ledge.postMessage(...).
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
// Only the page itself loads; a link can't take the window anywhere.
- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    BOOL page = [action.request.URL.absoluteString isEqualToString:@"about:blank"];
    decisionHandler(page ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
}
@end

static NSMutableDictionary<NSNumber *, NSWindow *> *windows;
static NSMutableDictionary<NSNumber *, WKWebView *> *webViews;
static NSMutableDictionary<NSNumber *, LedgeDelegate *> *delegates;

void ledgeInit(void) {
    onMainSync(^{
        windows = [NSMutableDictionary dictionary];
        webViews = [NSMutableDictionary dictionary];
        delegates = [NSMutableDictionary dictionary];
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
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

static WKWebView *newWebView(int window, NSRect frame) {
    LedgeDelegate *delegate = [LedgeDelegate new];
    delegate.windowID = window;
    WKWebViewConfiguration *config = [WKWebViewConfiguration new];
    [config.userContentController addScriptMessageHandler:delegate name:@"ledge"];
    WKWebView *webView = [[LedgeWebView alloc] initWithFrame:frame configuration:config];
    webView.navigationDelegate = delegate;
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
        win.contentView = newWebView(window, frame);
        windows[@(window)] = win;
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
