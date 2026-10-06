package tetra3d

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// listScene returns a scene with a bumpy grid in two mesh parts, so that the
// second part's vertex range does not start at 0. Some vertices are behind the
// camera. colors adds vertex colours, and texture gives the material a
// texture.
func listScene(colors, texture bool) (*Scene, *Camera, *Mesh) {
	const side = 60
	mesh := NewMesh("grid")
	for p := range 2 {
		mat := NewMaterial("grid")
		if texture {
			mat.Texture = ebiten.NewImage(64, 32)
		}
		part := mesh.AddMeshPart(mat)
		verts := make([]VertexInfo, 0, side*side)
		for j := range side {
			for i := range side {
				x := float32(i)/float32(side-1)*200 - 100 + float32(p)*30
				z := float32(j)/float32(side-1)*160 - 130
				y := float32(8 * math.Sin(float64(x)*0.3) * math.Cos(float64(z)*0.2))
				v := NewVertex(x, y, z, float32(i)/float32(side-1)*2-0.5, float32(j)/float32(side-1))
				if colors {
					v.Colors = append(v.Colors, Color4{float32(i) / side, float32(j) / side, 0.5, 1})
				}
				verts = append(verts, v)
			}
		}
		mesh.AddVertices(verts...)
		indices := make([]int, 0, (side-1)*(side-1)*6)
		for j := range side - 1 {
			for i := range side - 1 {
				a := j*side + i
				indices = append(indices, a, a+side, a+1, a+1, a+side, a+side+1)
			}
		}
		part.AddTriangles(indices...)
	}
	if colors {
		mesh.VertexActiveColorChannel = 0
	}
	mesh.UpdateDimensions()

	scene := NewScene("list")
	scene.World.LightingOn = false
	model := NewModel("grid", mesh)
	model.Color = Color4{0.9, 0.8, 0.7, 1}
	model.FrustumCulling = false
	camera := NewCamera("camera", 1280, 800)
	camera.SetLocalPosition(0, 4, 0)
	scene.Root.AddChildren(camera, model)
	return scene, camera, mesh
}

// renderLists renders scene and returns copies of the colour vertex list and
// the index list, which hold the last mesh part drawn. Camera.Render expects
// empty lists, and a test that calls processVertices alone leaves them in use.
func renderLists(scene *Scene, camera *Camera) ([]ebiten.Vertex, []uint16) {
	vertexListIndex, indexListIndex = 0, 0
	clear(colorVertexList)
	clear(indexList)
	camera.RenderScene(scene)
	return append([]ebiten.Vertex(nil), colorVertexList...), append([]uint16(nil), indexList...)
}

// TestWriteVertexListMatchesGeneralLoop checks that the plain loop of the
// vertex-list writes gives the same vertex and index lists as the general
// loop, to the bit. A custom depth function that returns the depth unchanged
// makes Camera.Render take the general loop.
func TestWriteVertexListMatchesGeneralLoop(t *testing.T) {
	for _, tc := range []struct {
		name            string
		colors, texture bool
	}{
		{"vertex colours and texture", true, true},
		{"vertex colours", true, false},
		{"no vertex colours", false, true},
	} {
		scene, camera, mesh := listScene(tc.colors, tc.texture)
		plainVerts, plainIndices := renderLists(scene, camera)
		for _, part := range mesh.MeshParts {
			part.Material.CustomDepthFunction = func(_ *Model, _ *Camera, _ *MeshPart, _ int, depth float32) float32 { return depth }
		}
		generalVerts, generalIndices := renderLists(scene, camera)

		drawn := 0
		for i := range generalVerts {
			a, b := plainVerts[i], generalVerts[i]
			fa := [...]float32{a.DstX, a.DstY, a.SrcX, a.SrcY, a.ColorR, a.ColorG, a.ColorB, a.ColorA, a.Custom0, a.Custom1, a.Custom2, a.Custom3}
			fb := [...]float32{b.DstX, b.DstY, b.SrcX, b.SrcY, b.ColorR, b.ColorG, b.ColorB, b.ColorA, b.Custom0, b.Custom1, b.Custom2, b.Custom3}
			for k := range fa {
				if math.Float32bits(fa[k]) != math.Float32bits(fb[k]) {
					t.Fatalf("%s: vertex %d: %+v, want %+v", tc.name, i, a, b)
				}
			}
			if b.ColorA != 0 {
				drawn++
			}
		}
		for i := range generalIndices {
			if plainIndices[i] != generalIndices[i] {
				t.Fatalf("%s: index %d: %d, want %d", tc.name, i, plainIndices[i], generalIndices[i])
			}
		}
		if drawn < 500 {
			t.Errorf("%s: %d vertices written, want at least 500", tc.name, drawn)
		}
	}
}

// TestWriteVertexListBounds checks that the bounds from the plain loop give
// the same rectangle as depthPassRect over the vertices that it wrote, also
// when a vertex is behind the camera and when a position is not a number.
func TestWriteVertexListBounds(t *testing.T) {
	model, camera := gridModel(60)
	vp := camera.ViewMatrix().Mult(camera.Projection())
	mesh := model.mesh
	part := mesh.MeshParts[0]
	vertexListIndex = 0
	model.processVertices(vp, camera, part, true, nil, false)
	for vertexListIndex >= len(colorVertexList) {
		growDisplayLists()
	}
	n := len(mesh.VertexPositions)
	nextVertexStamp()
	for _, st := range globalSortingTriangleBucket.sorted {
		tri := st.Triangle
		globalVertexStamp[tri.VertexIndexA] = globalVertexStampNow
		globalVertexStamp[tri.VertexIndexB] = globalVertexStampNow
		globalVertexStamp[tri.VertexIndexC] = globalVertexStampNow
	}

	behind := false
	for i := range n {
		if globalVertexStamp[i] == globalVertexStampNow && globalVertexTransforms[i].W < 0 {
			behind = true
		}
	}
	if !behind {
		t.Error("no drawn vertex is behind the camera")
	}

	write := func() (int, screenBounds) {
		return writeVertexList(colorVertexList, 0, 0, n, globalVertexStampNow, globalVertexStamp, globalVertexSlot,
			globalVertexTransforms, globalVertexScreen, mesh.VertexUVs, nil, Color4{1, 1, 1, 1},
			1280, 800, 0, 0, 0.1, 100)
	}

	count, bounds := write()
	gotRect, gotPartial := bounds.rect(1280, 800)
	wantRect, wantPartial := depthPassRect(colorVertexList[:count], 1280, 800)
	if gotRect != wantRect || gotPartial != wantPartial {
		t.Errorf("rectangle %v %v, want %v %v", gotRect, gotPartial, wantRect, wantPartial)
	}

	// Two parts in one list give the rectangle of both.
	joined := emptyScreenBounds()
	joined.add(screenBounds{minX: 10, minY: 20, maxX: 30, maxY: 40})
	joined.add(screenBounds{minX: 5, minY: 25, maxX: 20, maxY: 60})
	if rect, partial := joined.rect(1280, 800); rect.Min.X != 4 || rect.Min.Y != 19 || rect.Max.X != 32 || rect.Max.Y != 62 || !partial {
		t.Errorf("joined rectangle %v %v", rect, partial)
	}

	for i := range n {
		if globalVertexStamp[i] == globalVertexStampNow && globalVertexTransforms[i].W >= 0 {
			globalVertexScreen[i].X = float32(math.NaN())
			break
		}
	}
	count, bounds = write()
	gotRect, gotPartial = bounds.rect(1280, 800)
	wantRect, wantPartial = depthPassRect(colorVertexList[:count], 1280, 800)
	if gotRect != wantRect || gotPartial != wantPartial || gotPartial {
		t.Errorf("not a number: rectangle %v %v, want %v %v", gotRect, gotPartial, wantRect, wantPartial)
	}
}

// BenchmarkWriteVertexList measures the plain loop of the vertex-list writes
// over the drawn vertices of a mesh part of about 30,000 vertices.
func BenchmarkWriteVertexList(b *testing.B) {
	model, camera := gridModel(174)
	vp := camera.ViewMatrix().Mult(camera.Projection())
	mesh := model.mesh
	vertexListIndex = 0
	model.processVertices(vp, camera, mesh.MeshParts[0], true, nil, false)
	for vertexListIndex >= len(colorVertexList) {
		growDisplayLists()
	}
	n := len(mesh.VertexPositions)
	nextVertexStamp()
	for _, st := range globalSortingTriangleBucket.sorted {
		tri := st.Triangle
		globalVertexStamp[tri.VertexIndexA] = globalVertexStampNow
		globalVertexStamp[tri.VertexIndexB] = globalVertexStampNow
		globalVertexStamp[tri.VertexIndexC] = globalVertexStampNow
	}
	colors := make([]Color4, n)
	count := 0
	b.ResetTimer()
	for b.Loop() {
		count, _ = writeVertexList(colorVertexList, 0, 0, n, globalVertexStampNow, globalVertexStamp, globalVertexSlot,
			globalVertexTransforms, globalVertexScreen, mesh.VertexUVs, colors, Color4{1, 1, 1, 1},
			1280, 800, 0, 0, 0.1, 100)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(count), "ns/vertex")
}
