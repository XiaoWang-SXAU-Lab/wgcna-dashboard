//go:build windows

package subproc

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW prevents a console-subsystem child (Rscript.exe, the Rust
// worker) from allocating a visible console window when spawned from the GUI
// app. Combined with HideWindow it suppresses the brief terminal flashes.
const createNoWindow = 0x08000000

// HideWindow marks cmd so it starts without a visible console window.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
