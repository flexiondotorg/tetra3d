package tetra3d

import "testing"

// vertex returns a vertex at x with the given colours, bones, and weights.
func vertex(x float32, colors []Color4, bones []uint16, weights []float32) VertexInfo {
	v := NewVertex(x, 0, 0, 0, 0)
	v.NormalY = 1
	v.Colors = colors
	v.Bones = bones
	v.Weights = weights
	return v
}

// TestAddVerticesSizes checks that AddVertices keeps every per-vertex buffer
// at the vertex count: one colour channel for vertices with one colour, the
// lights, and the original normals, and no bones or weights for a mesh with
// none.
func TestAddVerticesSizes(t *testing.T) {
	m := NewMesh("t")
	red := []Color4{{1, 0, 0, 1}}
	for range 3 {
		m.AddVertices(vertex(1, red, nil, nil), vertex(2, red, nil, nil), vertex(3, red, nil, nil))
	}
	n := len(m.VertexPositions)
	if n != 9 {
		t.Fatalf("%d vertices, want 9", n)
	}
	if len(m.VertexColors) != 1 {
		t.Errorf("%d colour channels, want 1", len(m.VertexColors))
	}
	if c := m.VertexColors[0].colors; len(c) != n || c[n-1] != red[0] {
		t.Errorf("colour channel of %d, last %v, want %d, %v", len(c), c[len(c)-1], n, red[0])
	}
	if len(m.vertexLights.colors) != n {
		t.Errorf("%d vertex lights, want %d", len(m.vertexLights.colors), n)
	}
	if len(m.VertexNormalsOriginal) != n || m.VertexNormalsOriginal[n-1] != (Vector3{0, 1, 0}) {
		t.Errorf("%d original normals, want %d", len(m.VertexNormalsOriginal), n)
	}
	if len(m.VertexBones) != 0 || len(m.VertexWeights) != 0 {
		t.Errorf("%d bones and %d weights for a mesh with no bones, want none", len(m.VertexBones), len(m.VertexWeights))
	}
	if v := m.GetVertexInfo(n - 1); v.X != 3 || v.Bones != nil || v.Weights != nil {
		t.Errorf("GetVertexInfo gives %+v", v)
	}
}

// TestAddVerticesBones checks that the bones and the weights cover every
// vertex once a vertex has them, with none for the vertices before it.
func TestAddVerticesBones(t *testing.T) {
	m := NewMesh("t")
	m.AddVertices(vertex(1, nil, nil, nil), vertex(2, nil, nil, nil))
	m.AddVertices(vertex(3, nil, []uint16{4}, []float32{1}), vertex(4, nil, nil, nil))
	if len(m.VertexBones) != 4 || len(m.VertexWeights) != 4 {
		t.Fatalf("%d bones and %d weights, want 4 each", len(m.VertexBones), len(m.VertexWeights))
	}
	if m.VertexBones[0] != nil || m.VertexBones[2][0] != 4 || m.VertexWeights[2][0] != 1 {
		t.Errorf("bones %v, weights %v", m.VertexBones, m.VertexWeights)
	}
	if v := m.GetVertexInfo(2); len(v.Bones) != 1 || v.Bones[0] != 4 {
		t.Errorf("GetVertexInfo gives bones %v, want [4]", v.Bones)
	}
	if len(m.VertexColors) != 1 || len(m.VertexColors[0].colors) != 4 {
		t.Errorf("%d colour channels, want 1 in white for vertices with no colour", len(m.VertexColors))
	}
}
