// Package mmap provides a cross-platform interface for memory-mapped file I/O.
//
// It supports Linux, macOS, and Windows without cgo.
package mmap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// MMap represents a memory-mapped region. It is a named []byte type so callers
// can index, slice, and range over mapped memory directly.
type MMap []byte

// Prot describes memory protection flags.
type Prot int

const (
	ProtRead  Prot = 1 << iota // Pages may be read.
	ProtWrite                  // Pages may be written.
	ProtExec                   // Pages may be executed.

	ProtNone Prot = 0 // Pages may not be accessed.
)

// Flag describes mapping type flags.
type Flag int

const (
	MapShared    Flag = 1 << iota // Share changes with other processes.
	MapPrivate                    // Copy-on-write private mapping.
	MapAnonymous                  // Not backed by any file (fd is ignored).
)

// Advice describes memory access pattern hints for Advise.
type Advice int

const (
	AdvNormal     Advice = iota // No special treatment (default).
	AdvRandom                   // Expect random page references.
	AdvSequential               // Expect sequential page references.
	AdvWillNeed                 // Will need these pages soon.
	AdvDontNeed                 // Don't need these pages soon.
)

// SyncFlag controls the behavior of Flush.
type SyncFlag int

const (
	SyncAsync      SyncFlag = 1 << iota // Schedule flush, return immediately.
	SyncSync                            // Flush synchronously (block until done).
	SyncInvalidate                      // Invalidate other mappings of the same file.
)

// Sentinel errors.
var (
	ErrUnmapped   = errors.New("mmap: mapping already unmapped or invalid")
	ErrInvalidFD  = errors.New("mmap: invalid file descriptor for non-anonymous mapping")
	ErrZeroLength = errors.New("mmap: length must be greater than zero")
	ErrNotAligned = errors.New("mmap: offset must be page-aligned")
)

// Mapping registry: tracks original base pointer → length so that Unmap works
// correctly even if the caller resliced the MMap.
var (
	mu       sync.Mutex
	mappings = make(map[uintptr]int64)
)

func addMapping(b []byte) {
	if len(b) == 0 {
		return
	}
	key := pointerOf(b)
	mu.Lock()
	mappings[key] = int64(len(b))
	mu.Unlock()
}

func removeMapping(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	key := pointerOf(b)
	mu.Lock()
	n, ok := mappings[key]
	if ok {
		delete(mappings, key)
	}
	mu.Unlock()
	return n, ok
}

// MapFile maps an entire file for reading. The file is opened, its size
// determined, and mapped read-only with shared visibility.
// The caller must call Unmap when done.
//
// For block devices (e.g. /dev/sda1) where Stat does not report a size,
// MapFile falls back to seeking to determine the device size.
func MapFile(path string) (MMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("mmap: open: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("mmap: stat: %w", err)
	}

	size := fi.Size()
	if size == 0 {
		if fi.Mode().IsRegular() {
			return nil, ErrZeroLength
		}
		size, err = f.Seek(0, io.SeekEnd)
		if err != nil || size == 0 {
			return nil, ErrZeroLength
		}
	}

	return MapRegion(int(f.Fd()), size, ProtRead, MapShared, 0)
}

// MapRegion maps a region of a file descriptor (or anonymous memory) into the
// process address space.
//
// fd is the file descriptor to map. Pass -1 for anonymous mappings (must also
// set MapAnonymous in flags). length is the number of bytes to map and must be
// > 0. offset is the byte offset in the file where mapping begins and must be
// page-aligned.
func MapRegion(fd int, length int64, prot Prot, flags Flag, offset int64) (MMap, error) {
	if length <= 0 {
		return nil, ErrZeroLength
	}
	if offset < 0 {
		return nil, ErrNotAligned
	}

	pageSize := int64(os.Getpagesize())
	if offset%pageSize != 0 {
		return nil, ErrNotAligned
	}

	if fd < 0 && flags&MapAnonymous == 0 {
		return nil, ErrInvalidFD
	}

	b, err := mapRegion(fd, length, prot, flags, offset)
	if err != nil {
		return nil, err
	}

	addMapping(b)
	return MMap(b), nil
}

// Unmap removes the memory mapping. After Unmap returns successfully the MMap
// must not be used. Calling Unmap on an already-unmapped region returns
// ErrUnmapped.
func (m *MMap) Unmap() error {
	if m == nil || len(*m) == 0 {
		return ErrUnmapped
	}

	b := []byte(*m)
	if _, ok := removeMapping(b); !ok {
		return ErrUnmapped
	}

	err := unmapRegion(b)
	*m = nil
	return err
}

// Flush flushes changes made to the mapped region back to the underlying file.
func (m MMap) Flush(flags SyncFlag) error {
	if len(m) == 0 {
		return ErrUnmapped
	}
	return flushRegion([]byte(m), flags)
}

// Lock locks the mapped region in physical memory, preventing it from being
// paged to swap.
func (m MMap) Lock() error {
	if len(m) == 0 {
		return ErrUnmapped
	}
	return lockRegion([]byte(m))
}

// Unlock reverses a previous Lock, allowing the OS to page the memory.
func (m MMap) Unlock() error {
	if len(m) == 0 {
		return ErrUnmapped
	}
	return unlockRegion([]byte(m))
}

// Advise provides a hint to the kernel about expected access patterns for
// the mapped region. On Windows this is a no-op.
func (m MMap) Advise(advice Advice) error {
	if len(m) == 0 {
		return ErrUnmapped
	}
	return adviseRegion([]byte(m), advice)
}

// Reader provides sequential io.Reader, io.ReaderAt, io.WriterAt, io.Seeker,
// and io.Closer access over an MMap.
type Reader struct {
	data   MMap
	offset int64
}

var (
	_ io.Reader   = (*Reader)(nil)
	_ io.ReaderAt = (*Reader)(nil)
	_ io.WriterAt = (*Reader)(nil)
	_ io.Seeker   = (*Reader)(nil)
	_ io.Closer   = (*Reader)(nil)
)

// NewReader returns a new Reader over the given MMap.
func NewReader(m MMap) *Reader {
	return &Reader{data: m}
}

// Len returns the number of bytes in the underlying mapping.
func (r *Reader) Len() int {
	return len(r.data)
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if r.data == nil {
		return 0, ErrUnmapped
	}
	if r.offset >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += int64(n)
	if r.offset >= int64(len(r.data)) && n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// ReadAt implements io.ReaderAt.
func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	if r.data == nil {
		return 0, ErrUnmapped
	}
	if off < 0 || off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// WriteAt implements io.WriterAt. It writes to the mapped memory at the given
// offset. The mapping must have been created with ProtWrite.
func (r *Reader) WriteAt(p []byte, off int64) (int, error) {
	if r.data == nil {
		return 0, ErrUnmapped
	}
	if off < 0 || off >= int64(len(r.data)) {
		return 0, io.ErrShortWrite
	}
	n := copy(r.data[off:], p)
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// Seek implements io.Seeker.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.offset + offset
	case io.SeekEnd:
		abs = int64(len(r.data)) + offset
	default:
		return 0, errors.New("mmap.Reader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("mmap.Reader.Seek: negative position")
	}
	r.offset = abs
	return abs, nil
}

// Close calls Unmap on the underlying MMap. After Close the Reader must not be
// used.
func (r *Reader) Close() error {
	err := r.data.Unmap()
	r.data = nil
	return err
}
