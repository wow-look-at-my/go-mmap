//go:build !js && !wasip1

package mmap

import (
	"go/build"
	"testing"

	"github.com/stretchr/testify/require"
)

// A file without the constraint lets the package build.
func TestNoFileBuildsOnWasm(t *testing.T) {
	for _, goos := range []string{"js", "wasip1"} {
		ctx := build.Default
		ctx.GOOS, ctx.GOARCH = goos, "wasm"
		_, err := ctx.ImportDir(".", 0)
		var noGo *build.NoGoError
		require.ErrorAs(t, err, &noGo, "%s/wasm builds a file of this package", goos)
	}
}
