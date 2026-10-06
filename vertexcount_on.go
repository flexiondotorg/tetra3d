//go:build vertexcount

package tetra3d

// VertexCountHook, when not nil, receives the work of each visible mesh part
// that Camera.Render processes: the vertices in the range that the vertex
// pass transforms on the processor, the triangles that the triangle pass tests, and the
// triangles that it keeps to draw. DrawnTriangle gives the kept triangles
// during the call. Only builds with the vertexcount tag have the hook.
var VertexCountHook func(camera *Camera, model *Model, part *MeshPart, vertices, triangles, drawn int)

// DrawnTriangle returns kept triangle i, from 0 to drawn-1, of the mesh part
// that VertexCountHook receives.
func DrawnTriangle(i int) *Triangle {
	return globalSortingTriangleBucket.tris[globalSortingTriangleBucket.unsetTris[i].index]
}

// countPart reports the work of processVertices for part to VertexCountHook.
func countPart(camera *Camera, model *Model, part *MeshPart) {
	if VertexCountHook == nil || !model.visible || (model.Color.A == 0 && model.isTransparent(part)) {
		return
	}
	lo, hi := 0, len(model.mesh.VertexPositions)
	if !model.mesh.autoSubdivide {
		lo, hi = part.vertexRange()
	}
	VertexCountHook(camera, model, part, hi-lo, part.TriangleCount(), globalSortingTriangleBucket.unsetTriIndex)
}
