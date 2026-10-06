//go:build !vertexcount

package tetra3d

// countPart is empty without the vertexcount tag, see vertexcount_on.go.
func countPart(*Camera, *Model, *MeshPart) {}
