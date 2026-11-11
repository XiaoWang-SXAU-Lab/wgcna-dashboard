package main

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit
void configureFileDialogs(void *window);
*/
import "C"
import "unsafe"

func configureFileDialogs(window unsafe.Pointer) {
	C.configureFileDialogs(window)
}
