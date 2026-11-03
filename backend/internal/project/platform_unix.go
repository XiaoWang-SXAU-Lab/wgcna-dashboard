//go:build darwin || linux

package project

import (
	"os"
	"syscall"
)

func lockFile(f *os.File) error         { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func replaceFile(from, to string) error { return os.Rename(from, to) }
