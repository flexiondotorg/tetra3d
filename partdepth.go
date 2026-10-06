package tetra3d

import "github.com/hajimehoshi/ebiten/v2"

// sortedVertexSource is the Kage vertex function of the depth test inside a
// part, see Camera.DepthInParts. The CPU transform gives the pixel position
// in DstX and DstY, and the w of the clip position in Custom2. The function
// gives the GPU the pixel position times w, the clip z of the mesh path,
// SortedClip.x * (w - SortedClip.y), and w, see gpuClip. So the GPU interpolates every
// value correctly for the perspective, and its depth test compares the same
// depth as the mesh path.
const sortedVertexSource = `

var SortedClip vec2

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	w := custom.z
	return imageDstProjection() * vec4((dstPos+imageDstOrigin())*w, SortedClip.x*(w-SortedClip.y), w), srcPos + imageSrc0Origin(), color, custom
}
`

// withSortedVertex returns the shader source src with the vertex function of
// the depth test inside a part.
func withSortedVertex(src []byte) []byte {
	return append(append([]byte(nil), src...), sortedVertexSource...)
}

// sortedShaders holds the variant for the depth test inside a part of each
// shader that ExtendBase3DShader makes.
var sortedShaders = map[*ebiten.Shader]*ebiten.Shader{}

// partDepth reports whether the sorted path draws part of model with a depth
// test inside the part, see DepthInParts. A part that is transparent, that
// clips its alpha, or that moves its depth with a function keeps the sort.
func (camera *Camera) partDepth(model *Model, part *MeshPart) bool {
	if !camera.DepthInParts || !camera.perspective || !camera.RenderDepth || camera.RenderNormals ||
		camera.PerspectiveCorrectedTextureMapping || model.VertexClipFunction != nil || model.isTransparent(part) {
		return false
	}
	mat := part.Material
	if mat == nil {
		return true
	}
	if mat.TransparencyMode == TransparencyModeAlphaClip || mat.CustomDepthFunction != nil || mat.BillboardedDepthMode == DepthModeUnbillboarded {
		return false
	}
	return !mat.shaderOn() || sortedShaders[mat.fragmentShader] != nil
}

// depthUnits is the number of steps of the three-byte depth encoding of the
// depth shaders, 255 cubed.
const depthUnits = 255 * 65025

// depthGateSteps is how far, in steps of the depth encoding, the depth of a
// fragment of the colour pass can be from the depth that the depth pass kept.
// The two passes interpolate the depth in two programs, which can round
// apart. Eight steps are about 0.5 mm over a depth range of a kilometre.
const depthGateSteps = 8
