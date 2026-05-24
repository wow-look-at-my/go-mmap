//go:build linux || darwin

package mmap

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func pointerOf(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func protToUnix(p Prot) int {
	var flags int
	if p&ProtRead != 0 {
		flags |= unix.PROT_READ
	}
	if p&ProtWrite != 0 {
		flags |= unix.PROT_WRITE
	}
	if p&ProtExec != 0 {
		flags |= unix.PROT_EXEC
	}
	return flags
}

func flagToUnix(f Flag) int {
	var flags int
	if f&MapShared != 0 {
		flags |= unix.MAP_SHARED
	}
	if f&MapPrivate != 0 {
		flags |= unix.MAP_PRIVATE
	}
	if f&MapAnonymous != 0 {
		flags |= unix.MAP_ANON
	}
	return flags
}

func syncToUnix(s SyncFlag) int {
	var flags int
	if s&SyncAsync != 0 {
		flags |= unix.MS_ASYNC
	}
	if s&SyncSync != 0 {
		flags |= unix.MS_SYNC
	}
	if s&SyncInvalidate != 0 {
		flags |= unix.MS_INVALIDATE
	}
	return flags
}

func adviceToUnix(a Advice) int {
	switch a {
	case AdvRandom:
		return unix.MADV_RANDOM
	case AdvSequential:
		return unix.MADV_SEQUENTIAL
	case AdvWillNeed:
		return unix.MADV_WILLNEED
	case AdvDontNeed:
		return unix.MADV_DONTNEED
	default:
		return unix.MADV_NORMAL
	}
}

func mapRegion(fd int, length int64, prot Prot, flags Flag, offset int64) ([]byte, error) {
	b, err := unix.Mmap(fd, offset, int(length), protToUnix(prot), flagToUnix(flags))
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	return b, nil
}

func unmapRegion(b []byte) error {
	if err := unix.Munmap(b); err != nil {
		return fmt.Errorf("munmap: %w", err)
	}
	return nil
}

func flushRegion(b []byte, flags SyncFlag) error {
	if err := unix.Msync(b, syncToUnix(flags)); err != nil {
		return fmt.Errorf("msync: %w", err)
	}
	return nil
}

func lockRegion(b []byte) error {
	if err := unix.Mlock(b); err != nil {
		return fmt.Errorf("mlock: %w", err)
	}
	return nil
}

func unlockRegion(b []byte) error {
	if err := unix.Munlock(b); err != nil {
		return fmt.Errorf("munlock: %w", err)
	}
	return nil
}

func adviseRegion(b []byte, advice Advice) error {
	if err := unix.Madvise(b, adviceToUnix(advice)); err != nil {
		return fmt.Errorf("madvise: %w", err)
	}
	return nil
}
