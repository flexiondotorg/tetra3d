package tetra3d

import (
	"bytes"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestMeshColour checks that meshColour replaces the discard of the base 3D
// shader, with and without a custom fragment, and that the result compiles
// with each vertex function of the mesh path.
func TestMeshColour(t *testing.T) {
	custom := "//kage:unit pixels\npackage main\n\nfunc CustomFragment(dstPos vec4, srcPos vec2, color vec4) vec4 {\n\treturn color\n}\n"
	for _, c := range []string{"", custom} {
		src := meshColour(base3DShaderSource(c))
		if bytes.Contains(src, []byte("discard()")) {
			t.Errorf("meshColour left a discard with custom %q", c)
		}
		if !bytes.Contains(src, []byte("return vec4(0)")) {
			t.Errorf("meshColour returns no transparent pixel with custom %q", c)
		}
		for _, with := range []func([]byte) []byte{withGPUMesh, withGPURigid, withGPUBend, withGPUPose} {
			if _, err := ebiten.NewShader(with(src)); err != nil {
				t.Errorf("custom %q: %v", c, err)
			}
		}
	}
}
