package tetra3d

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2"
)

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

// meshColourHardware returns the base 3D shader source src for the colour
// pass of the mesh path with Camera.HardwareDepth: meshColour with no gate.
// The hardware depth test of the colour texture alone keeps the nearest
// part, so the shader reads no depth image, and the fog takes the depth of
// the fragment itself, which is the depth that the depth pass would keep.
func meshColourHardware(src []byte) []byte {
	src = meshColour(src)
	src = bytes.Replace(src, []byte("depth := imageSrc1UnsafeAtFromSrc0Pos(dstPosToSrcPos(dstPos.xy))"), []byte("depth := vec4(1)"), 1)
	return bytes.ReplaceAll(src, []byte("decodeDepth(depth)"), []byte("custom.y"))
}

// hardwareColour returns the base 3D shader source src for the colour pass
// of the sorted path with Camera.HardwareDepth: meshColourHardware, which
// reads no depth image, with a discard of each fragment whose texture alpha
// is at most AlphaClip, when AlphaClip is more than 0. So an alpha-clip part
// draws its colour and its depth in one draw, as the depth pass of the
// sorted path clips it. The vertex function is hardwareVertexSource.
func hardwareColour(src []byte) []byte {
	src = meshColourHardware(src)
	src = bytes.Replace(src, []byte("var DepthGate float\n"), []byte("var DepthGate float\n\n// AlphaClip, when it is more than 0, discards a fragment whose texture alpha\n// is at most AlphaClip, see Camera.HardwareDepth.\nvar AlphaClip float\n"), 1)
	return bytes.Replace(src, []byte("\t\t// tetra3d Custom Fragment Call Location //"), []byte("\t\tif AlphaClip > 0 && colorTex.a <= AlphaClip*color.a {\n\t\t\tdiscard()\n\t\t}\n\n\t\t// tetra3d Custom Fragment Call Location //"), 1)
}

// hardwareVertexSource is the Kage vertex function of the sorted path with
// Camera.HardwareDepth. The CPU transform gives the pixel position in DstX
// and DstY, the depth in Custom1, and the w of the clip position in Custom2.
// The function gives the GPU the pixel position times w, the clip z of the
// mesh path with Camera.HardwareDepth, HardwareClip.z * (w - HardwareClip.w)
// for the near plane HardwareClip.w, see newGPUClipPlanes, and w. So the
// parts of the sorted path test the same hardware depth as the mesh path.
//
// With HardwareCustom more than 0, for a part whose depth a custom depth
// function or the unbillboarded depth moves, the clip z takes the distance
// from the depth in Custom1, HardwareClip.x * depth + HardwareClip.y, in
// place of w.
const hardwareVertexSource = `

var HardwareClip vec4
var HardwareCustom float

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	w := custom.z
	d := w
	if HardwareCustom > 0 {
		d = custom.y*HardwareClip.x + HardwareClip.y
	}
	return imageDstProjection() * vec4((dstPos+imageDstOrigin())*w, HardwareClip.z*(w-HardwareClip.w*w/d), w), srcPos + imageSrc0Origin(), color, custom
}
`

// withHardwareVertex returns the shader source src with the vertex function
// of the sorted path with Camera.HardwareDepth.
func withHardwareVertex(src []byte) []byte {
	return append(append([]byte(nil), src...), hardwareVertexSource...)
}

// hardwareShaders holds the variant for Camera.HardwareDepth of each shader
// that ExtendBase3DShader makes.
var hardwareShaders = map[*ebiten.Shader]*ebiten.Shader{}
