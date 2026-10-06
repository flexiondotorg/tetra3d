package tetra3d

import (
	"testing"
	"unsafe"
)

// TestTriangleSize checks that a Triangle fits in the 80-byte size class
// of the Go allocator, because a scene holds millions of them.
func TestTriangleSize(t *testing.T) {
	if n := unsafe.Sizeof(Triangle{}); n > 80 {
		t.Errorf("a Triangle takes %d bytes, want at most 80", n)
	}
}
