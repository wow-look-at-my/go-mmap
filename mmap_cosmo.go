//go:build cosmo

package mmap

import (
	"fmt"
	"syscall"
	"unsafe"
)

// A Cosmopolitan binary runs on Linux, macOS, Windows and the BSDs, and its
// system call layer speaks the Linux ABI on every one of them. So the numbers
// below are Linux's, and they are the portable ones here. golang.org/x/sys/unix
// does not build for this target, and the standard syscall package carries only
// mmap and munmap, so the rest go through syscall.Syscall.
const (
	msAsync      = 0x1
	msInvalidate = 0x2
	msSync       = 0x4

	madvNormal     = 0x0
	madvRandom     = 0x1
	madvSequential = 0x2
	madvWillNeed   = 0x3
	madvDontNeed   = 0x4
)

func pointerOf(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func protToCosmo(p Prot) int {
	var flags int
	if p&ProtRead != 0 {
		flags |= syscall.PROT_READ
	}
	if p&ProtWrite != 0 {
		flags |= syscall.PROT_WRITE
	}
	if p&ProtExec != 0 {
		flags |= syscall.PROT_EXEC
	}
	return flags
}

func flagToCosmo(f Flag) int {
	var flags int
	if f&MapShared != 0 {
		flags |= syscall.MAP_SHARED
	}
	if f&MapPrivate != 0 {
		flags |= syscall.MAP_PRIVATE
	}
	if f&MapAnonymous != 0 {
		flags |= syscall.MAP_ANON
	}
	return flags
}

func syncToCosmo(s SyncFlag) int {
	var flags int
	if s&SyncAsync != 0 {
		flags |= msAsync
	}
	if s&SyncSync != 0 {
		flags |= msSync
	}
	if s&SyncInvalidate != 0 {
		flags |= msInvalidate
	}
	return flags
}

func adviceToCosmo(a Advice) int {
	switch a {
	case AdvRandom:
		return madvRandom
	case AdvSequential:
		return madvSequential
	case AdvWillNeed:
		return madvWillNeed
	case AdvDontNeed:
		return madvDontNeed
	default:
		return madvNormal
	}
}

// region calls one of the system calls that take an address, a length and one
// flag word. The caller has already refused an empty mapping.
func region(trap uintptr, name string, b []byte, arg int) error {
	_, _, errno := syscall.Syscall(trap, pointerOf(b), uintptr(len(b)), uintptr(arg))
	if errno != 0 {
		return fmt.Errorf("%s: %w", name, errno)
	}
	return nil
}

func mapRegion(fd int, length int64, prot Prot, flags Flag, offset int64) ([]byte, error) {
	b, err := syscall.Mmap(fd, offset, int(length), protToCosmo(prot), flagToCosmo(flags))
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	return b, nil
}

func unmapRegion(b []byte) error {
	if err := syscall.Munmap(b); err != nil {
		return fmt.Errorf("munmap: %w", err)
	}
	return nil
}

func flushRegion(b []byte, flags SyncFlag) error {
	return region(syscall.SYS_MSYNC, "msync", b, syncToCosmo(flags))
}

func lockRegion(b []byte) error {
	return region(syscall.SYS_MLOCK, "mlock", b, 0)
}

func unlockRegion(b []byte) error {
	return region(syscall.SYS_MUNLOCK, "munlock", b, 0)
}

func adviseRegion(b []byte, advice Advice) error {
	return region(syscall.SYS_MADVISE, "madvise", b, adviceToCosmo(advice))
}
