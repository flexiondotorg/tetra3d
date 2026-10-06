package tetra3d

import (
	"math"
	"slices"
	"testing"

	"github.com/solarlune/tetra3d/math32"
)

// refTri is a triangle that the triangle pass keeps, and its depth.
type refTri struct {
	tri   *Triangle
	depth float32
}

// referenceTrianglePass is the triangle pass of an unskinned model on the
// plain path, reading each triangle through its pointer. It uses the clip
// codes and the screen positions that processVertices left.
func referenceTrianglePass(model *Model, camera *Camera, part *MeshPart) []refTri {
	camPos := camera.WorldPosition()
	p, s, r := model.Transform().Inverted().Decompose()
	invertedCamPos := r.MultVec(camPos).Add(p.Mult(Vector3{1 / s.X, 1 / s.Y, 1 / s.Z}))
	backfaceCulling := part.Material != nil && part.Material.BackfaceCulling
	var kept []refTri
	for triIndex := part.TriangleStart; triIndex <= part.TriangleEnd; triIndex++ {
		tri := part.Mesh.Triangles[triIndex]
		a, b, c := tri.VertexIndexA, tri.VertexIndexB, tri.VertexIndexC
		if globalVertexClipCodes[a]&globalVertexClipCodes[b]&globalVertexClipCodes[c] != 0 {
			continue
		}
		if backfaceCulling {
			v0, v1, v2 := globalVertexScreen[a], globalVertexScreen[b], globalVertexScreen[c]
			if (v0.X-v1.X)*(v1.Y-v2.Y)-(v1.X-v2.X)*(v0.Y-v1.Y) < 0 {
				continue
			}
		}
		dx := invertedCamPos.X - tri.Center.X
		dy := invertedCamPos.Y - tri.Center.Y
		dz := invertedCamPos.Z - tri.Center.Z
		depth := float32(dx*dx + dy*dy + dz*dz)
		if math32.IsNaN(depth) || math32.IsInf(depth, 0) {
			continue
		}
		kept = append(kept, refTri{tri, depth})
	}
	return kept
}

// referenceSort sorts kept back to front into the depth bins of
// sortingTriangleBucket.Sort, keeping the order within each bin.
func referenceSort(kept []refTri, binCount int) []refTri {
	minDepth, maxDepth := float32(math.MaxFloat32), -float32(math.MaxFloat32)
	for _, k := range kept {
		minDepth = min(minDepth, k.depth)
		maxDepth = max(maxDepth, k.depth)
	}
	rangeDiff := maxDepth - minDepth
	if rangeDiff == 0 {
		rangeDiff += 0.001
	}
	bin := func(k refTri) int32 {
		return int32(math32.Clamp((k.depth-minDepth)/rangeDiff*float32(binCount), 0, float32(binCount-1)))
	}
	sorted := slices.Clone(kept)
	slices.SortStableFunc(sorted, func(x, y refTri) int { return int(bin(y) - bin(x)) })
	return sorted
}

// bucketTris returns the kept triangles of globalSortingTriangleBucket in the
// order of AddTriangle and in draw order.
func bucketTris() (added, sorted []refTri) {
	b := globalSortingTriangleBucket
	for _, st := range b.unsetTris[:b.unsetTriIndex] {
		added = append(added, refTri{b.tris[st.index], st.depth})
	}
	for _, st := range b.sorted {
		sorted = append(sorted, refTri{b.tris[st.index], st.depth})
	}
	return added, sorted
}

func sameTris(t *testing.T, what string, got, want []refTri) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d triangles, want %d", what, len(got), len(want))
	}
	for i := range want {
		if got[i].tri != want[i].tri || math.Float32bits(got[i].depth) != math.Float32bits(want[i].depth) {
			t.Fatalf("%s: triangle %d: %p %v, want %p %v", what, i, got[i].tri, got[i].depth, want[i].tri, want[i].depth)
		}
	}
}

// TestTrianglePassMatchesPointerReads checks that the triangle pass, which
// reads the packed triangle data, keeps the same triangles with the same
// depths, to the bit, and in the same draw order as reads through each
// triangle's pointer, with and without back-face culling.
func TestTrianglePassMatchesPointerReads(t *testing.T) {
	model, camera := gridModel(60)
	model.SetLocalPosition(1.5, -0.25, 3)
	model.SetLocalScale(1.1, 0.9, 1)
	model.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, 0.3))
	vp := camera.ViewMatrix().Mult(camera.Projection())
	part := model.mesh.MeshParts[0]
	for _, culling := range []bool{false, true} {
		part.Material.BackfaceCulling = culling
		vertexListIndex = 0
		globalSortingTriangleBucket.sortMode = TriangleSortModeBackToFront
		model.processVertices(vp, camera, part, true, nil, false)
		want := referenceTrianglePass(model, camera, part)
		added, sorted := bucketTris()
		sameTris(t, "added", added, want)
		sameTris(t, "sorted", sorted, referenceSort(want, globalSortingTriangleBucket.binCount))
		if len(want) < 500 || len(want) == part.TriangleCount() {
			t.Errorf("culling %v: %d of %d triangles kept, want some culled and at least 500 kept", culling, len(want), part.TriangleCount())
		}
	}
}

// TestTriangleDataFollowsChanges checks that the packed triangle data follows
// new triangles, Triangle.RecalculateCenter, and UpdateTriangleData after an
// in-place change of the vertex indices.
func TestTriangleDataFollowsChanges(t *testing.T) {
	model, camera := gridModel(20)
	mesh := model.mesh
	vp := camera.ViewMatrix().Mult(camera.Projection())
	part := mesh.MeshParts[0]
	check := func(what string) {
		t.Helper()
		vertexListIndex = 0
		model.processVertices(vp, camera, part, true, nil, false)
		added, _ := bucketTris()
		sameTris(t, what, added, referenceTrianglePass(model, camera, part))
	}
	check("first render")

	part.AddTriangles(0, 20, 1)
	check("added triangle")

	mesh.VertexPositions[21].Y += 3
	for _, tri := range mesh.Triangles {
		tri.RecalculateCenter()
	}
	check("recalculated centers")

	tri := mesh.Triangles[5]
	tri.VertexIndexA, tri.VertexIndexB = tri.VertexIndexB, tri.VertexIndexA
	mesh.UpdateTriangleData()
	check("changed indices")
}

// TestIndexListMatchesPointerReads checks that the vertex marks and the index
// writes, which read the packed vertex indices, mark the same vertices and
// give the same index list as reads through each drawn triangle's pointer.
func TestIndexListMatchesPointerReads(t *testing.T) {
	for _, texture := range []bool{false, true} {
		scene, camera, mesh := listScene(true, texture)
		_, indices := renderLists(scene, camera)

		// The lists and the bucket hold the last mesh part drawn.
		b := globalSortingTriangleBucket
		if len(b.sorted) < 500 {
			t.Fatalf("texture %v: %d triangles drawn, want at least 500", texture, len(b.sorted))
		}
		want := make([]bool, len(mesh.VertexPositions))
		for k, st := range b.sorted {
			tri := mesh.Triangles[st.index]
			for j := range 3 {
				v := tri.VertexIndex(j)
				want[v] = true
				if got := indices[3*k+j]; got != uint16(globalVertexSlot[v]) {
					t.Fatalf("texture %v: index %d: %d, want %d", texture, 3*k+j, got, globalVertexSlot[v])
				}
			}
		}
		for v := range want {
			if marked := globalVertexStamp[v] == globalVertexStampNow; marked != want[v] {
				t.Fatalf("texture %v: vertex %d marked %v, want %v", texture, v, marked, want[v])
			}
		}
	}
}
