package mmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectLines(t *testing.T, content string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	var got []string
	require.NoError(t, FileLines(path, func(line []byte) bool {
		got = append(got, string(line))
		return true
	}))
	return got
}

func TestFileLinesSplitsOnNewline(t *testing.T) {
	assert.Equal(t, []string{"one", "two", "", "three"}, collectLines(t, "one\ntwo\r\n\nthree"))
	assert.Equal(t, []string{"one", "two"}, collectLines(t, "one\ntwo\n"))
}

func TestFileLinesEmptyFileHasNoLines(t *testing.T) {
	assert.Empty(t, collectLines(t, ""))
}

// A line far past bufio.Scanner's default limit must come back whole.
func TestFileLinesHasNoLengthLimit(t *testing.T) {
	long := strings.Repeat("x", 3<<20)
	got := collectLines(t, "head\n"+long+"\ntail\n")
	require.Len(t, got, 3)
	assert.Equal(t, "head", got[0])
	assert.Len(t, got[1], len(long))
	assert.Equal(t, "tail", got[2])
}

func TestFileLinesStopsWhenVisitDeclines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("a\nb\nc\n"), 0o644))
	var got []string
	require.NoError(t, FileLines(path, func(line []byte) bool {
		got = append(got, string(line))
		return len(got) < 2
	}))
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestFileLinesMissingFileFails(t *testing.T) {
	err := FileLines(filepath.Join(t.TempDir(), "missing"), func([]byte) bool { return true })
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestScannerNext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("a\r\n\nlast"), 0o644))
	mapped, err := MapFile(path)
	require.NoError(t, err)
	defer mapped.Unmap()

	scan := mapped.Scanner()
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
	mapped, err := MapFile(path)
	require.NoError(t, err)
	defer mapped.Unmap()
	starts := []*byte{}
	for line := range mapped.Lines() {
		starts = append(starts, &line[0])
	}
	assert.Equal(t, []*byte{&mapped[0], &mapped[3]}, starts)
}
