package mmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectLines answers the lines FileLines visits, and false where the
// platform maps no memory.
func collectLines(t *testing.T, content string) ([]string, bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	var got []string
	err := FileLines(path, func(line []byte) bool {
		got = append(got, string(line))
		return true
	})
	return got, mapped(t, err)
}

func TestFileLinesSplitsOnNewline(t *testing.T) {
	got, ok := collectLines(t, "one\ntwo\r\n\nthree")
	if !ok {
		return
	}
	assert.Equal(t, []string{"one", "two", "", "three"}, got)
	got, _ = collectLines(t, "one\ntwo\n")
	assert.Equal(t, []string{"one", "two"}, got)
}

// An empty file needs no mapping, so it has no lines on every platform.
func TestFileLinesEmptyFileHasNoLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, nil, 0o644))
	require.NoError(t, FileLines(path, func([]byte) bool {
		t.Error("visit called for an empty file")
		return true
	}))
}

// A line far past bufio.Scanner's default limit must come back whole.
func TestFileLinesHasNoLengthLimit(t *testing.T) {
	long := strings.Repeat("x", 3<<20)
	got, ok := collectLines(t, "head\n"+long+"\ntail\n")
	if !ok {
		return
	}
	require.Len(t, got, 3)
	assert.Equal(t, "head", got[0])
	assert.Len(t, got[1], len(long))
	assert.Equal(t, "tail", got[2])
}

func TestFileLinesStopsWhenVisitDeclines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("a\nb\nc\n"), 0o644))
	var got []string
	err := FileLines(path, func(line []byte) bool {
		got = append(got, string(line))
		return len(got) < 2
	})
	if !mapped(t, err) {
		return
	}
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestFileLinesMissingFileFails(t *testing.T) {
	err := FileLines(filepath.Join(t.TempDir(), "missing"), func([]byte) bool { return true })
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestScannerNext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("a\r\n\nlast"), 0o644))
	mapping, err := MapFile(path)
	if !mapped(t, err) {
		return
	}
	defer mapping.Unmap()

	scan := mapping.Scanner()
	for _, want := range []string{"a", "", "last"} {
		line, ok := scan.Next()
		require.True(t, ok)
		assert.Equal(t, want, string(line))
	}
	line, ok := scan.Next()
	assert.False(t, ok)
	assert.Nil(t, line)
}

func TestLinesAreSubslicesOfTheMapping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("ab\ncd\n"), 0o644))
	mapping, err := MapFile(path)
	if !mapped(t, err) {
		return
	}
	defer mapping.Unmap()
	starts := []*byte{}
	for line := range mapping.Lines() {
		starts = append(starts, &line[0])
	}
	assert.Equal(t, []*byte{&mapping[0], &mapping[3]}, starts)
}
