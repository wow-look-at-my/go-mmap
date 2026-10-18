//go:build js || wasip1

package mmap

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// WebAssembly has one linear memory and no mmap. A mapping is a heap copy of
// the file range. A shared writable mapping writes its bytes back through its
// descriptor on Flush and on Unmap, so that descriptor must stay open until
// Unmap. No other process sees the bytes before then.

func pointerOf(b []byte) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

// fileBacking is the file range a shared writable mapping writes back to.
type fileBacking struct {
	fd     int
	offset int64
	length int
}

var (
	backMu   sync.Mutex
	backings = make(map[uintptr]fileBacking)
)

func mapRegion(fd int, length int64, prot Prot, flags Flag, offset int64) ([]byte, error) {
	b := make([]byte, length)
	if flags&MapAnonymous != 0 {
		return b, nil
	}
	for done := 0; done < len(b); {
		n, err := syscall.Pread(fd, b[done:], offset+int64(done))
		if err != nil {
			return nil, fmt.Errorf("mmap: pread: %w", err)
		}
		if n == 0 {
			break // Past the end of the file, a mapping reads as zeros.
		}
		done += n
	}
	if flags&MapShared != 0 && prot&ProtWrite != 0 {
		backMu.Lock()
		backings[pointerOf(b)] = fileBacking{fd: fd, offset: offset, length: len(b)}
		backMu.Unlock()
	}
	return b, nil
}

// backingOf finds the mapping that holds b, which can be a subslice of it.
// It answers the descriptor and the file offset of the first byte of b.
func backingOf(b []byte) (int, int64, bool) {
	start := pointerOf(b)
	backMu.Lock()
	defer backMu.Unlock()
	for base, back := range backings {
		if start >= base && start < base+uintptr(back.length) {
			return back.fd, back.offset + int64(start-base), true
		}
	}
	return 0, 0, false
}

func unmapRegion(b []byte) error {
	fd, offset, ok := backingOf(b)
	if !ok {
		return nil
	}
	backMu.Lock()
	delete(backings, pointerOf(b))
	backMu.Unlock()
	if err := writeBack(fd, offset, b); err != nil {
		return fmt.Errorf("munmap: %w", err)
	}
	return nil
}

func flushRegion(b []byte, flags SyncFlag) error {
	fd, offset, ok := backingOf(b)
	if !ok {
		return nil
	}
	if err := writeBack(fd, offset, b); err != nil {
		return fmt.Errorf("msync: %w", err)
	}
	if flags&SyncSync != 0 {
		if err := syscall.Fsync(fd); err != nil {
			return fmt.Errorf("msync: fsync: %w", err)
		}
	}
	return nil
}

func writeBack(fd int, offset int64, b []byte) error {
	for done := 0; done < len(b); {
		n, err := syscall.Pwrite(fd, b[done:], offset+int64(done))
		if err != nil {
			return fmt.Errorf("pwrite: %w", err)
		}
		done += n
	}
	return nil
}

// The heap has no swap and no pager, so the remaining calls have nothing to do.

func lockRegion(b []byte) error { return nil }

func unlockRegion(b []byte) error { return nil }

func adviseRegion(b []byte, advice Advice) error { return nil }
