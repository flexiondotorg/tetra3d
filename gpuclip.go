package tetra3d

// The clip and depth conventions of the GPU vertex functions: the mesh path
// (gpuMeshSource and gpuRigidSource) and the depth test inside parts
// (sortedVertexSource). Each gives the GPU the pixel position times w, a
// clip z from 0 at the near plane to w at the far plane, and w, and passes
// on the depth that the CPU transform puts in Custom1.

// gpuClip holds the w of the clip position at the near and the far plane of
// the CPU transform, and the factor of the clip z: the clip z is
// k * (w - near). So a point at the near plane has a clip z of 0, and one at
// the far plane has a clip z of w, the clip range of every graphics
// library. OpenGL keeps a clip z down to -w, so it clips at about half the
// near distance.
type gpuClip struct {
	near, far, k float32
}

// newGPUClip returns the clip planes of a perspective camera. The CPU
// transform keeps a vertex whose clip z is from near-1 to far.
func newGPUClip(proj Matrix4, near, far float32) gpuClip {
	// The clip z is zv * proj[2][2] + proj[3][2] and the w is
	// zv * proj[2][3] + proj[3][3], for a view-space z of zv.
	w := func(clipZ float32) float32 {
		zv := (clipZ - proj[3][2]) / proj[2][2]
		return zv*proj[2][3] + proj[3][3]
	}
	c := gpuClip{near: w(near - 1), far: w(far)}
	c.k = c.far / (c.far - c.near)
	return c
}

// setGPUUniforms writes the uniforms of the mesh path for a mesh part with
// the model-view-projection matrix mvp, on a camWidth by camHeight target.
// GPUVertexMatrix takes a position to the pixel position times w, the clip
// z, and w. GPUVertexDepth takes it to the depth that the CPU transform puts
// in Custom1. GPUVertexTint is the colour of the model and the material, and
// GPUVertexTexSize is the size of the texture in pixels. See gpuMeshSource.
func (s *drawScratch) setGPUUniforms(mvp *Matrix4, clip gpuClip, camWidth, camHeight int, depthMargin, camSpread float32, tint Color4, srcW, srcH float32) {
	w, h := float32(camWidth), float32(camHeight)
	m := s.gpuMatrix
	// The matrix is in column-major order: m[4*c+r] is the factor of input c
	// in output r. Column j of mvp gives the clip coordinate j.
	for c := range 4 {
		x, y, z, cw := mvp[c][0], mvp[c][1], mvp[c][2], mvp[c][3]
		m[4*c] = x*w + cw*w/2
		m[4*c+1] = -y*h + cw*h/2
		m[4*c+2] = clip.k * cw
		m[4*c+3] = cw
		s.gpuDepth[c] = z / camSpread
	}
	m[14] -= clip.k * clip.near
	s.gpuDepth[3] += depthMargin / camSpread
	s.gpuTint[0], s.gpuTint[1], s.gpuTint[2], s.gpuTint[3] = tint.R, tint.G, tint.B, tint.A
	s.gpuTexSize[0], s.gpuTexSize[1] = srcW, srcH
}
