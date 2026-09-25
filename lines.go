package mmap

import (
	"bytes"
	"iter"
	"os"
)

// Lines yields each line of m without its line ending, "\n" or "\r\n". A final
// line with no newline is yielded too. Each line is a subslice of the mapping,
// so a line costs no copy and has no length limit. A line is valid only until
// Unmap.
func (m MMap) Lines() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		rest := []byte(m)
		for len(rest) > 0 {
			line := rest
			rest = nil
			if idx := bytes.IndexByte(line, '\n'); idx >= 0 {
				line, rest = line[:idx], line[idx+1:]
			}
			if !yield(bytes.TrimSuffix(line, []byte{'\r'})) {
				return
			}
		}
	}
}

// FileLines maps the file at path and calls visit with each of its lines, as
// Lines yields them. It stops when visit answers false, and unmaps the file
// before it returns. An empty regular file has no lines. A line must not be
// kept after visit returns, because the mapping is gone then.
func FileLines(path string, visit func(line []byte) bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() && info.Size() == 0 {
		return nil
	}
	mapped, err := MapFile(path)
	if err != nil {
		return err
	}
	for line := range mapped.Lines() {
		if !visit(line) {
			break
		}
	}
	return mapped.Unmap()
}
