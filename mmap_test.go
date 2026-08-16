package mmap

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "mmap-test-*")
	require.Nil(t, err)

	_, err = f.Write(data)
	require.Nil(t, err)

	name := f.Name()
	f.Close()
	return name
}

func TestMapFile(t *testing.T) {
	want := []byte("hello, mmap!")
	path := tempFile(t, want)

	m, err := MapFile(path)
	require.Nil(t, err)

	defer m.Unmap()

	require.True(t, bytes.Equal([]byte(m), want))

}

func TestMapFileEmpty(t *testing.T) {
	path := tempFile(t, nil)

	_, err := MapFile(path)
	require.NotNil(t, err)

}

func TestMapFileNotExist(t *testing.T) {
	_, err := MapFile(filepath.Join(t.TempDir(), "nonexistent"))
	require.NotNil(t, err)

}

func TestMapRegionReadOnly(t *testing.T) {
	want := []byte("read-only data here")
	path := tempFile(t, want)

	f, err := os.Open(path)
	require.Nil(t, err)

	defer f.Close()

	m, err := MapRegion(int(f.Fd()), int64(len(want)), ProtRead, MapShared, 0)
	require.Nil(t, err)

	defer m.Unmap()

	require.True(t, bytes.Equal([]byte(m), want))

}

func TestMapRegionReadWrite(t *testing.T) {
	initial := []byte("initial data!!")
	path := tempFile(t, initial)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.Nil(t, err)

	defer f.Close()

	m, err := MapRegion(int(f.Fd()), int64(len(initial)), ProtRead|ProtWrite, MapShared, 0)
	require.Nil(t, err)

	// Modify the mapping.
	copy(m, []byte("CHANGED DATA!!"))

	require.NoError(t, m.Flush(SyncSync))

	require.NoError(t, m.Unmap())

	// Verify the file was modified.
	got, err := os.ReadFile(path)
	require.Nil(t, err)

	require.True(t, bytes.Equal(got, []byte("CHANGED DATA!!")))

}

func TestMapRegionPrivate(t *testing.T) {
	initial := []byte("private test data")
	path := tempFile(t, initial)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.Nil(t, err)

	defer f.Close()

	m, err := MapRegion(int(f.Fd()), int64(len(initial)), ProtRead|ProtWrite, MapPrivate, 0)
	require.Nil(t, err)

	// Write to the private mapping.
	copy(m, []byte("XXXXXXXXXXXXXXXXX"))
	m.Unmap()

	// Original file should be unchanged.
	got, err := os.ReadFile(path)
	require.Nil(t, err)

	require.True(t, bytes.Equal(got, initial))

}

func TestAnonymousMapping(t *testing.T) {
	size := int64(os.Getpagesize())
	m, err := MapRegion(-1, size, ProtRead|ProtWrite, MapPrivate|MapAnonymous, 0)
	require.Nil(t, err)

	defer m.Unmap()

	// Write and read back.
	for i := range m {
		m[i] = byte(i % 256)
	}
	for i := range m {
		require.Equal(t, byte(i%256), m[i])

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
	require.Nil(t, err)

	defer f.Close()

	// Map only the second page.
	m, err := MapRegion(int(f.Fd()), int64(pageSize), ProtRead, MapShared, int64(pageSize))
	require.Nil(t, err)

	defer m.Unmap()

	require.Equal(t, 'B', m[0])

}

func TestFlush(t *testing.T) {
	data := []byte("flush test data!")
	path := tempFile(t, data)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.Nil(t, err)

	defer f.Close()

	m, err := MapRegion(int(f.Fd()), int64(len(data)), ProtRead|ProtWrite, MapShared, 0)
	require.Nil(t, err)

	defer m.Unmap()

	copy(m, []byte("FLUSH TEST DONE!"))

	require.NoError(t, m.Flush(SyncSync))

}

func TestLockUnlock(t *testing.T) {
	data := []byte("lock test")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	defer m.Unmap()

	require.NoError(t, m.Lock())

	require.NoError(t, m.Unlock())

}

func TestAdvise(t *testing.T) {
	data := []byte("advise test data")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	defer m.Unmap()

	for _, adv := range []Advice{AdvNormal, AdvRandom, AdvSequential, AdvWillNeed, AdvDontNeed} {
		require.NoError(t, m.Advise(adv))

	}
}

func TestUnmap(t *testing.T) {
	data := []byte("unmap test")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	require.NoError(t, m.Unmap())

	// Second unmap should return ErrUnmapped.
	err = m.Unmap()
	require.Equal(t, ErrUnmapped, err)

}

func TestZeroLength(t *testing.T) {
	_, err := MapRegion(-1, 0, ProtRead, MapPrivate|MapAnonymous, 0)
	require.Equal(t, ErrZeroLength, err)

}

func TestInvalidOffset(t *testing.T) {
	data := make([]byte, os.Getpagesize()*2)
	path := tempFile(t, data)

	f, err := os.Open(path)
	require.Nil(t, err)

	defer f.Close()

	// Non-page-aligned offset.
	_, err = MapRegion(int(f.Fd()), 100, ProtRead, MapShared, 1)
	require.Equal(t, ErrNotAligned, err)

}

func TestInvalidFD(t *testing.T) {
	_, err := MapRegion(-1, 100, ProtRead, MapShared, 0)
	require.Equal(t, ErrInvalidFD, err)

}

func TestReaderRead(t *testing.T) {
	data := []byte("reader test data 1234567890")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	r := NewReader(m)
	defer r.Close()

	require.Equal(t, len(data), r.Len())

	buf := make([]byte, 6)
	n, err := r.Read(buf)
	require.Nil(t, err)

	require.False(t, n != 6 || string(buf) != "reader")

	// Read the rest.
	rest, err := io.ReadAll(r)
	require.Nil(t, err)

	require.Equal(t, " test data 1234567890", string(rest))

}

func TestReaderReadAt(t *testing.T) {
	data := []byte("abcdefghijklmnop")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	r := NewReader(m)
	defer r.Close()

	buf := make([]byte, 4)
	n, err := r.ReadAt(buf, 4)
	require.Nil(t, err)

	require.False(t, n != 4 || string(buf) != "efgh")

}

func TestReaderWriteAt(t *testing.T) {
	data := []byte("writeable data!!")
	path := tempFile(t, data)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.Nil(t, err)

	defer f.Close()

	m, err := MapRegion(int(f.Fd()), int64(len(data)), ProtRead|ProtWrite, MapShared, 0)
	require.Nil(t, err)

	r := NewReader(m)
	defer r.Close()

	n, err := r.WriteAt([]byte("WRITTEN"), 0)
	require.Nil(t, err)

	require.Equal(t, 7, n)

	require.Equal(t, "WRITTEN", string(m[:7]))

}

func TestReaderSeek(t *testing.T) {
	data := []byte("seek test data")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	r := NewReader(m)
	defer r.Close()

	// Seek to offset 5.
	pos, err := r.Seek(5, io.SeekStart)
	require.Nil(t, err)

	require.Equal(t, int64(5), pos)

	buf := make([]byte, 4)
	r.Read(buf)
	require.Equal(t, "test", string(buf))

	// Seek from end.
	pos, err = r.Seek(-4, io.SeekEnd)
	require.Nil(t, err)

	require.Equal(t, int64(len(data))-4, pos)

	r.Read(buf)
	require.Equal(t, "data", string(buf))

}

func TestReaderSeekNegative(t *testing.T) {
	data := []byte("x")
	path := tempFile(t, data)

	m, err := MapFile(path)
	require.Nil(t, err)

	r := NewReader(m)
	defer r.Close()

	_, err = r.Seek(-1, io.SeekStart)
	require.NotNil(t, err)

}

func TestOperationsOnUnmapped(t *testing.T) {
	var m MMap

	err := m.Flush(SyncSync)
	require.Equal(t, ErrUnmapped, err)

	err = m.Lock()
	require.Equal(t, ErrUnmapped, err)

	err = m.Unlock()
	require.Equal(t, ErrUnmapped, err)

	err = m.Advise(AdvNormal)
	require.Equal(t, ErrUnmapped, err)

}
