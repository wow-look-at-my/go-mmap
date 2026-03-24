package mmap

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "mmap-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	return name
}

func TestMapFile(t *testing.T) {
	want := []byte("hello, mmap!")
	path := tempFile(t, want)

	m, err := MapFile(path)
	if err != nil {
		t.Fatalf("MapFile: %v", err)
	}
	defer m.Unmap()

	if !bytes.Equal([]byte(m), want) {
		t.Fatalf("got %q, want %q", m, want)
	}
}

func TestMapFileEmpty(t *testing.T) {
	path := tempFile(t, nil)

	_, err := MapFile(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestMapFileNotExist(t *testing.T) {
	_, err := MapFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestMapRegionReadOnly(t *testing.T) {
	want := []byte("read-only data here")
	path := tempFile(t, want)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := MapRegion(int(f.Fd()), len(want), ProtRead, MapShared, 0)
	if err != nil {
		t.Fatalf("MapRegion: %v", err)
	}
	defer m.Unmap()

	if !bytes.Equal([]byte(m), want) {
		t.Fatalf("got %q, want %q", m, want)
	}
}

func TestMapRegionReadWrite(t *testing.T) {
	initial := []byte("initial data!!")
	path := tempFile(t, initial)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := MapRegion(int(f.Fd()), len(initial), ProtRead|ProtWrite, MapShared, 0)
	if err != nil {
		t.Fatalf("MapRegion: %v", err)
	}

	// Modify the mapping.
	copy(m, []byte("CHANGED DATA!!"))

	if err := m.Flush(SyncSync); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if err := m.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	// Verify the file was modified.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("CHANGED DATA!!")) {
		t.Fatalf("file content after write: %q", got)
	}
}

func TestMapRegionPrivate(t *testing.T) {
	initial := []byte("private test data")
	path := tempFile(t, initial)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := MapRegion(int(f.Fd()), len(initial), ProtRead|ProtWrite, MapPrivate, 0)
	if err != nil {
		t.Fatalf("MapRegion: %v", err)
	}

	// Write to the private mapping.
	copy(m, []byte("XXXXXXXXXXXXXXXXX"))
	m.Unmap()

	// Original file should be unchanged.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, initial) {
		t.Fatalf("file should be unchanged, got %q", got)
	}
}

func TestAnonymousMapping(t *testing.T) {
	size := os.Getpagesize()
	m, err := MapRegion(-1, size, ProtRead|ProtWrite, MapPrivate|MapAnonymous, 0)
	if err != nil {
		t.Fatalf("MapRegion anonymous: %v", err)
	}
	defer m.Unmap()

	// Write and read back.
	for i := range m {
		m[i] = byte(i % 256)
	}
	for i := range m {
		if m[i] != byte(i%256) {
			t.Fatalf("byte %d: got %d, want %d", i, m[i], byte(i%256))
		}
	}
}

func TestMapRegionWithOffset(t *testing.T) {
	pageSize := os.Getpagesize()

	// Create a file with 2 pages of data.
	data := make([]byte, pageSize*2)
	for i := range data {
		if i < pageSize {
			data[i] = 'A'
		} else {
			data[i] = 'B'
		}
	}
	path := tempFile(t, data)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Map only the second page.
	m, err := MapRegion(int(f.Fd()), pageSize, ProtRead, MapShared, int64(pageSize))
	if err != nil {
		t.Fatalf("MapRegion with offset: %v", err)
	}
	defer m.Unmap()

	if m[0] != 'B' {
		t.Fatalf("expected 'B' at offset 0 of second page, got %q", m[0])
	}
}

func TestFlush(t *testing.T) {
	data := []byte("flush test data!")
	path := tempFile(t, data)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := MapRegion(int(f.Fd()), len(data), ProtRead|ProtWrite, MapShared, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Unmap()

	copy(m, []byte("FLUSH TEST DONE!"))

	if err := m.Flush(SyncSync); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestLockUnlock(t *testing.T) {
	data := []byte("lock test")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Unmap()

	if err := m.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := m.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestAdvise(t *testing.T) {
	data := []byte("advise test data")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Unmap()

	for _, adv := range []Advice{AdvNormal, AdvRandom, AdvSequential, AdvWillNeed, AdvDontNeed} {
		if err := m.Advise(adv); err != nil {
			t.Fatalf("Advise(%d): %v", adv, err)
		}
	}
}

func TestUnmap(t *testing.T) {
	data := []byte("unmap test")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	// Second unmap should return ErrUnmapped.
	if err := m.Unmap(); err != ErrUnmapped {
		t.Fatalf("second Unmap: got %v, want ErrUnmapped", err)
	}
}

func TestZeroLength(t *testing.T) {
	_, err := MapRegion(-1, 0, ProtRead, MapPrivate|MapAnonymous, 0)
	if err != ErrZeroLength {
		t.Fatalf("got %v, want ErrZeroLength", err)
	}
}

func TestInvalidOffset(t *testing.T) {
	data := make([]byte, os.Getpagesize()*2)
	path := tempFile(t, data)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Non-page-aligned offset.
	_, err = MapRegion(int(f.Fd()), 100, ProtRead, MapShared, 1)
	if err != ErrNotAligned {
		t.Fatalf("got %v, want ErrNotAligned", err)
	}
}

func TestInvalidFD(t *testing.T) {
	_, err := MapRegion(-1, 100, ProtRead, MapShared, 0)
	if err != ErrInvalidFD {
		t.Fatalf("got %v, want ErrInvalidFD", err)
	}
}

func TestReaderRead(t *testing.T) {
	data := []byte("reader test data 1234567890")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(m)
	defer r.Close()

	if r.Len() != len(data) {
		t.Fatalf("Len: got %d, want %d", r.Len(), len(data))
	}

	buf := make([]byte, 6)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 6 || string(buf) != "reader" {
		t.Fatalf("Read: got %d %q", n, buf)
	}

	// Read the rest.
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(rest) != " test data 1234567890" {
		t.Fatalf("ReadAll: got %q", rest)
	}
}

func TestReaderReadAt(t *testing.T) {
	data := []byte("abcdefghijklmnop")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(m)
	defer r.Close()

	buf := make([]byte, 4)
	n, err := r.ReadAt(buf, 4)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != 4 || string(buf) != "efgh" {
		t.Fatalf("ReadAt: got %d %q", n, buf)
	}
}

func TestReaderWriteAt(t *testing.T) {
	data := []byte("writeable data!!")
	path := tempFile(t, data)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := MapRegion(int(f.Fd()), len(data), ProtRead|ProtWrite, MapShared, 0)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(m)
	defer r.Close()

	n, err := r.WriteAt([]byte("WRITTEN"), 0)
	if err != nil {
		t.Fatalf("WriteAt: %v", err)
	}
	if n != 7 {
		t.Fatalf("WriteAt: wrote %d bytes", n)
	}

	if string(m[:7]) != "WRITTEN" {
		t.Fatalf("WriteAt: mapping shows %q", m[:7])
	}
}

func TestReaderSeek(t *testing.T) {
	data := []byte("seek test data")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(m)
	defer r.Close()

	// Seek to offset 5.
	pos, err := r.Seek(5, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if pos != 5 {
		t.Fatalf("Seek: got pos %d", pos)
	}

	buf := make([]byte, 4)
	r.Read(buf)
	if string(buf) != "test" {
		t.Fatalf("after seek read: got %q", buf)
	}

	// Seek from end.
	pos, err = r.Seek(-4, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek end: %v", err)
	}
	if pos != int64(len(data))-4 {
		t.Fatalf("Seek end: got pos %d", pos)
	}

	r.Read(buf)
	if string(buf) != "data" {
		t.Fatalf("after seek-end read: got %q", buf)
	}
}

func TestReaderSeekNegative(t *testing.T) {
	data := []byte("x")
	path := tempFile(t, data)

	m, err := MapFile(path)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(m)
	defer r.Close()

	_, err = r.Seek(-1, io.SeekStart)
	if err == nil {
		t.Fatal("expected error for negative seek")
	}
}

func TestOperationsOnUnmapped(t *testing.T) {
	var m MMap

	if err := m.Flush(SyncSync); err != ErrUnmapped {
		t.Fatalf("Flush on nil: got %v", err)
	}
	if err := m.Lock(); err != ErrUnmapped {
		t.Fatalf("Lock on nil: got %v", err)
	}
	if err := m.Unlock(); err != ErrUnmapped {
		t.Fatalf("Unlock on nil: got %v", err)
	}
	if err := m.Advise(AdvNormal); err != ErrUnmapped {
		t.Fatalf("Advise on nil: got %v", err)
	}
}
