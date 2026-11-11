#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// WKWebView holds both delegates weakly. The upstream UI delegate has already
// been autoreleased by window creation, so install and retain our own delegate.
@interface WGCNAFileDialogs : NSObject <WKUIDelegate, WKNavigationDelegate, WKDownloadDelegate>
@property(nonatomic, strong) NSMapTable<WKDownload *, NSDictionary *> *transfers;
@end

@implementation WGCNAFileDialogs
- (void)dealloc {
    [_transfers release];
    [super dealloc];
}

- (void)webView:(WKWebView *)webView runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters
    initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(NSArray<NSURL *> *))completionHandler {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseFiles = YES;
    panel.canChooseDirectories = parameters.allowsDirectories;
    panel.allowsMultipleSelection = parameters.allowsMultipleSelection;
    [panel beginSheetModalForWindow:webView.window completionHandler:^(NSModalResponse result) {
        completionHandler(result == NSModalResponseOK ? panel.URLs : nil);
    }];
}

- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    if (@available(macOS 11.3, *)) {
        if (action.shouldPerformDownload) {
            decisionHandler(WKNavigationActionPolicyDownload);
            return;
        }
    }
    decisionHandler(WKNavigationActionPolicyAllow);
}

- (void)webView:(WKWebView *)webView decidePolicyForNavigationResponse:(WKNavigationResponse *)response
    decisionHandler:(void (^)(WKNavigationResponsePolicy))decisionHandler {
    NSString *disposition = @"";
    if ([response.response isKindOfClass:[NSHTTPURLResponse class]]) {
        disposition = [(NSHTTPURLResponse *)response.response valueForHTTPHeaderField:@"Content-Disposition"] ?: @"";
    }
    if (!response.canShowMIMEType || [disposition.lowercaseString hasPrefix:@"attachment"]) {
        if (@available(macOS 11.3, *)) {
            decisionHandler(WKNavigationResponsePolicyDownload);
        } else {
            // Older WebKit has no WKDownload API; use the system browser.
            [[NSWorkspace sharedWorkspace] openURL:response.response.URL];
            decisionHandler(WKNavigationResponsePolicyCancel);
        }
        return;
    }
    decisionHandler(WKNavigationResponsePolicyAllow);
}

- (void)webView:(WKWebView *)webView navigationAction:(WKNavigationAction *)action
    didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    download.delegate = self;
}

- (void)webView:(WKWebView *)webView navigationResponse:(WKNavigationResponse *)response
    didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    download.delegate = self;
}

- (void)download:(WKDownload *)download decideDestinationUsingResponse:(NSURLResponse *)response
    suggestedFilename:(NSString *)filename completionHandler:(void (^)(NSURL *))completionHandler API_AVAILABLE(macos(11.3)) {
    NSSavePanel *panel = [NSSavePanel savePanel];
    panel.nameFieldStringValue = filename.lastPathComponent;
    panel.canCreateDirectories = YES;
    [panel beginSheetModalForWindow:download.webView.window completionHandler:^(NSModalResponse result) {
        if (result != NSModalResponseOK) {
            completionHandler(nil);
            return;
        }
        // WebKit requires a new path. Stage beside the destination so replacing
        // an existing file happens only after the whole download succeeds.
        NSURL *staged = [[panel.URL URLByDeletingLastPathComponent]
            URLByAppendingPathComponent:[@".rswgcna-download-" stringByAppendingString:NSUUID.UUID.UUIDString]];
        [self.transfers setObject:@{@"staged": staged, @"destination": panel.URL} forKey:download];
        completionHandler(staged);
    }];
}

- (void)downloadDidFinish:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    NSDictionary *transfer = [self.transfers objectForKey:download];
    if (!transfer) return;
    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *error = nil;
    NSURL *destination = transfer[@"destination"];
    if ([fm fileExistsAtPath:destination.path]) {
        [fm replaceItemAtURL:destination withItemAtURL:transfer[@"staged"] backupItemName:nil
            options:0 resultingItemURL:nil error:&error];
    } else {
        [fm moveItemAtURL:transfer[@"staged"] toURL:destination error:&error];
    }
    if (error) [[NSAlert alertWithError:error] runModal];
    [fm removeItemAtURL:transfer[@"staged"] error:NULL];
    [self.transfers removeObjectForKey:download];
}

- (void)download:(WKDownload *)download didFailWithError:(NSError *)error
    resumeData:(NSData *)resumeData API_AVAILABLE(macos(11.3)) {
    NSDictionary *transfer = [self.transfers objectForKey:download];
    if (transfer) [[NSFileManager defaultManager] removeItemAtURL:transfer[@"staged"] error:NULL];
    [self.transfers removeObjectForKey:download];
    if (error.code != NSURLErrorCancelled) [[NSAlert alertWithError:error] runModal];
}
@end

static WKWebView *findWebView(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) return (WKWebView *)view;
    for (NSView *child in view.subviews) {
        WKWebView *found = findWebView(child);
        if (found) return found;
    }
    return nil;
}

void configureFileDialogs(void *handle) {
    NSWindow *window = (__bridge NSWindow *)handle;
    WKWebView *webView = findWebView(window.contentView);
    if (!webView) return;
    WGCNAFileDialogs *dialogs = [WGCNAFileDialogs new];
    dialogs.transfers = [NSMapTable strongToStrongObjectsMapTable];
    webView.UIDelegate = dialogs;
    webView.navigationDelegate = dialogs;
    objc_setAssociatedObject(webView, @selector(configureFileDialogs), dialogs, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    [dialogs release];
}
