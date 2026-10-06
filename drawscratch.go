package tetra3d

import (
	"maps"

	"github.com/hajimehoshi/ebiten/v2"
)

// drawScratch holds the values that Camera.Render reuses from one draw call
// and one frame to the next, so that the draw calls make no allocations for
// their uniforms, options, and render lists.
//
// Each uniform map gets its keys and values once, when it is made. Every value
// is a slice, and the draw calls write the new values into the slices, so the
// maps and their interface values never change. Ebitengine reads a slice
// whose length equals the dword count of the uniform in the same way as a
// scalar, so the uniform data is the same as with a new map of scalars.
type drawScratch struct {
	solids, transparents []renderPair

	colorShaderOptions ebiten.DrawTrianglesShaderOptions
	colorOptions       ebiten.DrawTrianglesOptions
	depthOptions       ebiten.DrawTrianglesShaderOptions
	clipOptions        ebiten.DrawTrianglesShaderOptions

	// worldUniforms is for a scene with a World, plainUniforms for a scene
	// without one, clipUniforms for the alpha-clip depth pass, and
	// fragmentUniforms for the colour pass of a material with its own
	// fragment shader uniforms.
	worldUniforms, plainUniforms, clipUniforms, fragmentUniforms map[string]any

	fog, fogRange, ditherSize, fogCurve, fogless []float32
	textureMapScreenSizeW, textureMapScreenSizeH []float32

	perspectiveCorrection, textureFilterMode, textureMapMode []int

	// The uniforms of the vertex function of the mesh path, see
	// setGPUUniforms, in every map. The shaders of the CPU transform have no
	// such uniforms, and Ebitengine ignores them.
	gpuMatrix, gpuDepth, gpuTint, gpuTexSize []float32

	// The uniforms of the vertex function of a model with a Bend, see
	// gpuBendSource and setBendUniforms, in every map.
	bendFirst, bendSecond, bendStretch []float32

	// The uniform of the vertex function of the depth test inside a part, in
	// every map, see sortedVertexSource, and the depth gate of the colour
	// pass, see base3d.kage.
	sortedClip, depthGate []float32

	// The parts, the cull uniform, and the options of the mesh path. The
	// first queued parts of meshDraws come from Camera.QueueMeshes.
	meshDraws                          []meshDraw
	queued                             int
	rigid                              []rigidPart
	rigidRecords                       []ebiten.Vertex
	meshCull                           []float32
	meshDepthOptions, meshColorOptions ebiten.DrawTrianglesShaderOptions

	// The quad and the options of Camera.clearForMeshes.
	clearVertices [4]ebiten.Vertex
	clearOptions  ebiten.DrawTrianglesShaderOptions

	// foglessValue holds fogless. foglessNormal is the value of Fogless in a
	// normal render, an int like the literal 1 that it stands for.
	foglessValue, foglessNormal any
}

func newDrawScratch() *drawScratch {
	s := &drawScratch{
		fog:                   make([]float32, 4),
		fogRange:              make([]float32, 2),
		ditherSize:            make([]float32, 1),
		fogCurve:              make([]float32, 1),
		fogless:               make([]float32, 1),
		textureMapScreenSizeW: make([]float32, 1),
		textureMapScreenSizeH: make([]float32, 1),
		perspectiveCorrection: make([]int, 1),
		textureFilterMode:     make([]int, 1),
		textureMapMode:        make([]int, 1),
		fragmentUniforms:      map[string]any{},
		gpuMatrix:             make([]float32, 16),
		gpuDepth:              make([]float32, 4),
		gpuTint:               make([]float32, 4),
		gpuTexSize:            make([]float32, 2),
		meshCull:              make([]float32, 4),
		bendFirst:             make([]float32, 16),
		bendSecond:            make([]float32, 16),
		bendStretch:           make([]float32, 4),
		sortedClip:            make([]float32, 2),
		depthGate:             make([]float32, 1),
	}
	s.foglessValue = s.fogless
	s.foglessNormal = []int{1}
	s.worldUniforms = map[string]any{
		"Fog":                             s.fog,
		"FogRange":                        s.fogRange,
		"DitherSize":                      s.ditherSize,
		"FogCurve":                        s.fogCurve,
		"BayerMatrix":                     bayerMatrix,
		"PerspectiveCorrection":           s.perspectiveCorrection,
		"TextureFilterMode":               s.textureFilterMode,
		"TextureMapMode":                  s.textureMapMode,
		"TextureMapScreenSizeMultiplierW": s.textureMapScreenSizeW,
		"TextureMapScreenSizeMultiplierH": s.textureMapScreenSizeH,
		"Fogless":                         s.foglessValue,
		"DepthGate":                       s.depthGate,
	}
	s.plainUniforms = map[string]any{
		"Fog":                   []float32{0, 0, 0, 0},
		"FogRange":              []float32{0, 1},
		"PerspectiveCorrection": s.perspectiveCorrection,
		"Fogless":               s.foglessValue,
		"DepthGate":             s.depthGate,
	}
	s.clipUniforms = map[string]any{
		"PerspectiveCorrection":           s.perspectiveCorrection,
		"TextureMapMode":                  s.textureMapMode,
		"TextureMapScreenSizeMultiplierW": s.textureMapScreenSizeW,
		"TextureMapScreenSizeMultiplierH": s.textureMapScreenSizeH,
		"TextureFilterMode":               s.textureFilterMode,
	}
	s.clipOptions.Uniforms = s.clipUniforms
	gpu := map[string]any{}
	for _, m := range []map[string]any{s.worldUniforms, s.plainUniforms, s.clipUniforms, gpu} {
		m["GPUVertexMatrix"] = s.gpuMatrix
		m["GPUVertexDepth"] = s.gpuDepth
		m["GPUVertexTint"] = s.gpuTint
		m["GPUVertexTexSize"] = s.gpuTexSize
		m["GPUMeshCull"] = s.meshCull
		m["SortedClip"] = s.sortedClip
		m["GPUBendFirst"] = s.bendFirst
		m["GPUBendSecond"] = s.bendSecond
		m["GPUBendStretch"] = s.bendStretch
	}
	s.depthOptions.Uniforms = gpu
	s.meshDepthOptions.Uniforms = gpu
	s.meshDepthOptions.Depth = true
	s.meshColorOptions.Depth = true
	return s
}

// setBendUniforms writes the uniforms of gpuBendSource for b. Column j of a
// Kage mat4 is row j of a Matrix4, so that the mat4 applies to a point as
// Matrix4.MultVec does.
func (s *drawScratch) setBendUniforms(b *Bend) {
	for r := range 4 {
		copy(s.bendFirst[4*r:4*r+4], b.First[r][:])
		copy(s.bendSecond[4*r:4*r+4], b.Second[r][:])
	}
	s.bendStretch[0], s.bendStretch[1], s.bendStretch[2] = b.Along.X, b.Along.Y, b.Along.Z
	s.bendStretch[3] = max(b.Stretch-1, 0)
}

// setPartUniforms writes the uniform values of one mesh part, which the
// alpha-clip depth pass and the colour pass share.
func (s *drawScratch) setPartUniforms(perspectiveCorrection, textureFilterMode, textureMapMode int, textureMapScreenSizeW, textureMapScreenSizeH float32) {
	s.perspectiveCorrection[0] = perspectiveCorrection
	s.textureFilterMode[0] = textureFilterMode
	s.textureMapMode[0] = textureMapMode
	s.textureMapScreenSizeW[0] = textureMapScreenSizeW
	s.textureMapScreenSizeH[0] = textureMapScreenSizeH
}

// colorUniforms returns the uniform map of the colour pass for world, which
// can be nil, with Fogless set to fogless, or to the int 1 in a normal render.
func (s *drawScratch) colorUniforms(world *World, fogless float32, normals bool) map[string]any {
	m := s.plainUniforms
	if world != nil {
		m = s.worldUniforms
		s.fog[0] = float32(world.FogColor.R)
		s.fog[1] = float32(world.FogColor.G)
		s.fog[2] = float32(world.FogColor.B)
		s.fog[3] = float32(world.FogMode)
		if !world.FogOn {
			s.fog[3] = -1
		}
		// FogRange goes to Ebitengine as the World's own slice would, so a
		// slice of another length still fails in the same way.
		if len(world.FogRange) != len(s.fogRange) {
			s.fogRange = make([]float32, len(world.FogRange))
			m["FogRange"] = s.fogRange
		}
		copy(s.fogRange, world.FogRange)
		s.ditherSize[0] = world.DitheredFogSize
		s.fogCurve[0] = float32(world.FogCurve)
	}
	s.fogless[0] = fogless
	if normals {
		m["Fogless"] = s.foglessNormal
	} else {
		m["Fogless"] = s.foglessValue
	}
	return m
}

// withFragmentUniforms returns a map with the entries of base and then of
// extra, the uniforms of a material's fragment shader. It reuses one map, so
// that the keys of one material do not stay in base for the next part.
func (s *drawScratch) withFragmentUniforms(base, extra map[string]any) map[string]any {
	clear(s.fragmentUniforms)
	maps.Copy(s.fragmentUniforms, base)
	maps.Copy(s.fragmentUniforms, extra)
	return s.fragmentUniforms
}
