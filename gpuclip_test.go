package tetra3d

import (
	"math"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestGPUUniformsMatchProcessorPath checks that the uniforms of the mesh
// path, applied on the processor, give the screen position and
// the depth that the CPU transform writes for each vertex in view, and a
// clip z from 0 to w between the near and the far plane.
func TestGPUUniformsMatchProcessorPath(t *testing.T) {
	scene, camera, mesh := listScene(true, false)
	vertices, _ := renderLists(scene, camera)
	model := scene.Root.Get("grid").(*Model)
	vp := camera.ViewMatrix().Mult(camera.Projection())
	mvp := model.Transform().Mult(vp)
	w, h := camera.Size()
	margin := (camera.far - camera.near) * camera.DepthMargin
	spread := camera.far - camera.near + margin*2
	clip := newGPUClip(camera.Projection(), camera.near, camera.far)
	s := newDrawScratch()
	s.setGPUUniforms(&mvp, clip, w, h, margin, spread, Color4{1, 1, 1, 1}, 0, 0)

	// The lists hold the last mesh part drawn.
	lo, hi := mesh.MeshParts[1].vertexRange()
	checked := 0
	for i := lo; i < hi; i++ {
		if globalVertexStamp[i] != globalVertexStampNow || globalVertexTransforms[i].W <= 0 {
			continue
		}
		p := mesh.VertexPositions[i]
		in := [4]float32{p.X, p.Y, p.Z, 1}
		var out [4]float32
		var depth float32
		for c := range 4 {
			for r := range 4 {
				out[r] += s.gpuMatrix[4*c+r] * in[c]
			}
			depth += s.gpuDepth[c] * in[c]
		}
		v := vertices[globalVertexSlot[i]]
		x, y := out[0]/out[3], out[1]/out[3]
		if math.Abs(float64(x-v.DstX)) > 0.01 || math.Abs(float64(y-v.DstY)) > 0.01 || math.Abs(float64(depth-v.Custom1)) > 1e-5 {
			t.Fatalf("vertex %d: %v %v %v, want %v %v %v", i, x, y, depth, v.DstX, v.DstY, v.Custom1)
		}
		// The CPU transform keeps a clip z from near-1 to far.
		z := globalVertexTransforms[i].Z
		if inRange := out[2] >= 0 && out[2] <= out[3]; inRange != (z+1 >= camera.near && z <= camera.far) {
			t.Fatalf("vertex %d: clip z %v of w %v at clip z %v", i, out[2], out[3], z)
		}
		checked++
	}
	if checked < 500 {
		t.Fatalf("%d vertices checked, want at least 500", checked)
	}
}

// TestRenderMeshesMatchesSortedPath draws a cube once as a model on the sorted
// path and once from a MeshBatch with one record that places it in the same
// way, and checks that the colour and the depth textures agree. The sorted
// path interpolates the depth linearly across the screen, and the mesh path
// with the perspective, so the depths agree exactly only on a face that is
// square to the camera.
func TestRenderMeshesMatchesSortedPath(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("cube")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	mesh := NewCubeMesh(2, 2, 2)
	mesh.MeshParts[0].Material.Color = NewColor4(1, 0.5, 0, 1)
	mesh.MeshParts[0].Material.Shadeless = true
	model := NewModel("cube", mesh)
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam, model)
	inFrame(t, mesh.BuildGPUMesh)
	spread := float64(cam.far-cam.near) * float64(1+2*cam.DepthMargin)
	metres := func(p []byte) float64 {
		return (float64(p[0])*65025 + float64(p[1])*255 + float64(p[2])) / depthUnits * spread
	}

	for _, c := range []struct {
		name     string
		yaw      float32
		maxDepth float64 // The largest depth difference, in metres.
	}{
		{"square", 0, 0.001},
		{"turned", 0.5, 0.5},
	} {
		const x, y, z, scale = 0.5, 0.3, -7, 1.5
		model.SetLocalPosition(x, y, z)
		model.SetLocalScale(scale, scale, scale)
		model.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, c.yaw))
		batches := []MeshBatch{{Mesh: mesh, Records: []ebiten.Vertex{{
			DstX: x, DstY: z, Custom1: y,
			SrcX: float32(math.Cos(float64(c.yaw))), SrcY: float32(math.Sin(float64(c.yaw))),
			Custom0: scale, ColorR: 1, ColorG: 1, ColorB: 1,
		}}}}
		read := func(onMeshPath bool) (colour, depth []byte, draws, instances int) {
			colour, depth = make([]byte, 4*64*64), make([]byte, 4*64*64)
			inFrame(t, func() {
				cam.Clear()
				model.SetVisible(!onMeshPath, false)
				if onMeshPath {
					cam.RenderMeshes(scene, batches)
				}
				cam.RenderScene(scene)
				cam.ColorTexture().ReadPixels(colour)
				cam.DepthTexture().ReadPixels(depth)
				draws, instances = cam.MeshStats()
			})
			return
		}
		sortedColour, sortedDepth, _, _ := read(false)
		meshColour, meshDepth, draws, instances := read(true)
		if draws != 2 || instances != 1 {
			t.Errorf("%s: %d mesh draws and %d instances, want 2 and 1", c.name, draws, instances)
		}

		// The two paths rasterise the edges of the cube on their own, so a
		// few pixels of the outline can differ.
		covered, colourDiff, depthDiff := 0, 0, 0
		for i := 0; i < len(sortedColour); i += 4 {
			if slices.Equal(sortedColour[i:i+4], sortedColour[:4]) && slices.Equal(meshColour[i:i+4], meshColour[:4]) {
				continue
			}
			covered++
			for ch := range 4 {
				if d := int(sortedColour[i+ch]) - int(meshColour[i+ch]); d < -2 || d > 2 {
					colourDiff++
					break
				}
			}
			if math.Abs(metres(sortedDepth[i:i+4])-metres(meshDepth[i:i+4])) > c.maxDepth {
				depthDiff++
			}
		}
		if covered < 300 {
			t.Fatalf("%s: the cube covers %d pixels, want at least 300", c.name, covered)
		}
		if colourDiff > covered/20 || depthDiff > covered/20 {
			t.Errorf("%s: %d colour and %d depth pixels differ of %d covered, want at most %d", c.name, colourDiff, depthDiff, covered, covered/20)
		}
	}
}
