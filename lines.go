package mmap

import (
	"bytes"
	"iter"
	"os"
)

// Scanner returns the lines of a mapping on demand, and reads no further than
// the line it returns.
type Scanner struct {
	rest []byte
}

// Scanner answers a Scanner positioned at the start of m.
func (m MMap) Scanner() *Scanner {
	return &Scanner{rest: m}
}

// Next answers the next line without its line ending, or false at the end of
// the mapping. The line is a subslice of the mapping and is valid until Unmap.
func (s *Scanner) Next() ([]byte, bool) {
	if len(s.rest) == 0 {
		return nil, false
	}
	line := s.rest
	s.rest = nil
	if idx := bytes.IndexByte(line, '\n'); idx >= 0 {
		line, s.rest = line[:idx], line[idx+1:]
	}
	return bytes.TrimSuffix(line, []byte{'\r'}), true
}

// Lines yields the lines of m as Scanner.Next returns them.
func (m MMap) Lines() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		scan := m.Scanner()
		for line, ok := scan.Next(); ok; line, ok = scan.Next() {
			if !yield(line) {
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
