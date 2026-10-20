//go:build cosmo || darwin || linux || windows

package mmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// supportedConstraint names the platforms this package builds for.
const supportedConstraint = "//go:build cosmo || darwin || linux || windows"

// Each file whose name names no platform carries supportedConstraint.
func TestFilesNameTheSupportedPlatforms(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, file := range files {
		base := strings.TrimSuffix(strings.TrimSuffix(file, ".go"), "_test")
		if strings.HasSuffix(base, "_cosmo") || strings.HasSuffix(base, "_darwin") || strings.HasSuffix(base, "_linux") || strings.HasSuffix(base, "_windows") {
			continue
		}
		src, err := os.ReadFile(file)
		require.NoError(t, err)
		first, _, _ := strings.Cut(string(src), "\n")
		first = strings.TrimSuffix(first, "\r")
		if strings.HasPrefix(first, "//go:build ") && first != supportedConstraint {
			// A file for some of the platforms, such as mmap_unix.go.
			continue
		}
		assert.Equal(t, supportedConstraint, first, "%s builds on platforms this package does not support", file)
	}
}
