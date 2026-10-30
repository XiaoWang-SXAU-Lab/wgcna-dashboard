//go:build !darwin

package main

import "unsafe"

// WebView2 and WebKitGTK provide their own file and download dialogs.
func configureFileDialogs(window unsafe.Pointer) {}
