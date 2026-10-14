//go:build !linux && !darwin && !windows && !cosmo

package mmap

import (
	"errors"
	"unsafe"
)

// This platform maps no memory. Every call fails, so a caller falls back to plain reads.

// supported reports whether this platform maps memory.
const supported = false

func pointerOf(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func mapRegion(int, int64, Prot, Flag, int64) ([]byte, error) {
	return nil, errors.ErrUnsupported
}

func unmapRegion([]byte) error { return errors.ErrUnsupported }

func flushRegion([]byte, SyncFlag) error { return errors.ErrUnsupported }

func lockRegion([]byte) error { return errors.ErrUnsupported }

func unlockRegion([]byte) error { return errors.ErrUnsupported }

func adviseRegion([]byte, Advice) error { return errors.ErrUnsupported }
