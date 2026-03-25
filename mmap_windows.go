//go:build windows

package mmap

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

func pointerOf(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

// windowsMapping stores the file mapping handle and the original file handle
// so they can be cleaned up on Unmap.
type windowsMapping struct {
	mapHandle  windows.Handle
	fileHandle windows.Handle // only set for non-anonymous mappings where we need FlushFileBuffers
}

var (
	handleMu sync.Mutex
	handles  = make(map[uintptr]windowsMapping)
)

func protToWindows(p Prot, f Flag) (flProtect uint32, dwAccess uint32) {
	if f&MapPrivate != 0 {
		// Copy-on-write: PAGE_WRITECOPY + FILE_MAP_COPY
		return windows.PAGE_WRITECOPY, windows.FILE_MAP_COPY
	}
	switch {
	case p&ProtWrite != 0 && p&ProtExec != 0:
		return windows.PAGE_EXECUTE_READWRITE, windows.FILE_MAP_WRITE | windows.FILE_MAP_EXECUTE
	case p&ProtExec != 0:
		return windows.PAGE_EXECUTE_READ, windows.FILE_MAP_READ | windows.FILE_MAP_EXECUTE
	case p&ProtWrite != 0:
		return windows.PAGE_READWRITE, windows.FILE_MAP_WRITE
	default:
		return windows.PAGE_READONLY, windows.FILE_MAP_READ
	}
}

func mapRegion(fd int, length int, prot Prot, flags Flag, offset int64) ([]byte, error) {
	flProtect, dwAccess := protToWindows(prot, flags)

	var fHandle windows.Handle
	if fd == -1 || flags&MapAnonymous != 0 {
		fHandle = windows.InvalidHandle
	} else {
		fHandle = windows.Handle(uintptr(fd))
	}

	// CreateFileMapping maxSize covers the mapped region from the offset.
	maxSize := uint64(offset) + uint64(length)
	maxSizeHigh := uint32(maxSize >> 32)
	maxSizeLow := uint32(maxSize & 0xFFFFFFFF)

	h, err := windows.CreateFileMapping(fHandle, nil, flProtect, maxSizeHigh, maxSizeLow, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateFileMapping: %w", err)
	}

	offsetHigh := uint32(uint64(offset) >> 32)
	offsetLow := uint32(uint64(offset) & 0xFFFFFFFF)

	addr, err := windows.MapViewOfFile(h, dwAccess, offsetHigh, offsetLow, uintptr(length))
	if err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("MapViewOfFile: %w", err)
	}

	b := unsafe.Slice((*byte)(unsafe.Pointer(addr)), length)

	wm := windowsMapping{mapHandle: h}
	if fHandle != windows.InvalidHandle {
		wm.fileHandle = fHandle
	}

	key := uintptr(unsafe.Pointer(&b[0]))
	handleMu.Lock()
	handles[key] = wm
	handleMu.Unlock()

	return b, nil
}

func unmapRegion(b []byte) error {
	key := uintptr(unsafe.Pointer(&b[0]))

	handleMu.Lock()
	wm, ok := handles[key]
	if ok {
		delete(handles, key)
	}
	handleMu.Unlock()

	addr := uintptr(unsafe.Pointer(&b[0]))
	if err := windows.UnmapViewOfFile(addr); err != nil {
		return fmt.Errorf("UnmapViewOfFile: %w", err)
	}

	if ok {
		if err := windows.CloseHandle(wm.mapHandle); err != nil {
			return fmt.Errorf("CloseHandle: %w", err)
		}
	}

	return nil
}

func flushRegion(b []byte, flags SyncFlag) error {
	addr := uintptr(unsafe.Pointer(&b[0]))
	if err := windows.FlushViewOfFile(addr, uintptr(len(b))); err != nil {
		return fmt.Errorf("FlushViewOfFile: %w", err)
	}

	// For synchronous flush, also flush the file buffers to disk.
	if flags&SyncSync != 0 {
		key := uintptr(unsafe.Pointer(&b[0]))
		handleMu.Lock()
		wm, ok := handles[key]
		handleMu.Unlock()
		if ok && wm.fileHandle != 0 && wm.fileHandle != windows.InvalidHandle {
			if err := windows.FlushFileBuffers(wm.fileHandle); err != nil {
				return fmt.Errorf("FlushFileBuffers: %w", err)
			}
		}
	}

	return nil
}

func lockRegion(b []byte) error {
	addr := uintptr(unsafe.Pointer(&b[0]))
	if err := windows.VirtualLock(addr, uintptr(len(b))); err != nil {
		return fmt.Errorf("VirtualLock: %w", err)
	}
	return nil
}

func unlockRegion(b []byte) error {
	addr := uintptr(unsafe.Pointer(&b[0]))
	if err := windows.VirtualUnlock(addr, uintptr(len(b))); err != nil {
		return fmt.Errorf("VirtualUnlock: %w", err)
	}
	return nil
}

func adviseRegion(_ []byte, _ Advice) error {
	// Windows has no madvise equivalent. This is a no-op.
	return nil
}
