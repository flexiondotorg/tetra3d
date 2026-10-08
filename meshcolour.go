package tetra3d

import "bytes"

// meshColour returns the base 3D shader source src for the colour pass of
// the mesh path. A pixel that the depth pass left empty returns transparent
// black in place of a discard. With BlendSourceOver, or another blend that
// keeps the destination for a transparent source, the draw leaves the pixel
// as it was. With no discard, the GPU can test the depth before the
// fragment shader runs.
//
// The transparent fragment still writes its depth into the hardware depth
// buffer of the colour pass. On the mesh path, the gate of the colour pass
// is the same for every part at a pixel: where the depth pass kept no part,
// every part returns transparent, and where it kept a part, no part returns
// transparent. So that depth hides no fragment that must show.
//
// The gate takes a depth only where its alpha is 1. depthIntermediate has
// an alpha of 0 or 1, and with Camera.GPUMeshDirectDepth, the depth texture
// has an alpha of 1 only where the depth pass kept a part.
func meshColour(src []byte) []byte {
	src = bytes.Replace(src, []byte("if depth.a > 0 &&"), []byte("if depth.a > 0.998 &&"), 1)
	return bytes.Replace(src, []byte("\n\tdiscard()\n\n}\n"), []byte("\n\treturn vec4(0)\n\n}\n"), 1)
}
