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
// square to the camera. It also checks that GPUMeshDirectDepth gives the
// same colour as the mesh path without it, and the same depth to within
// one step of the encoding.
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
		read := func(onMeshPath, direct bool) (colour, depth []byte, draws, instances int) {
			colour, depth = make([]byte, 4*64*64), make([]byte, 4*64*64)
			cam.GPUMeshDirectDepth = direct
			defer func() { cam.GPUMeshDirectDepth = false }()
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
		sortedColour, sortedDepth, _, _ := read(false, false)
		meshColour, meshDepth, draws, instances := read(true, false)
		// With GPUMeshDirectDepth, the depth pass draws into the depth
		// texture itself, which holds no hardware depth before it.
		directColour, directDepth, _, _ := read(true, true)
		// The depth shader differs, so its encoding can round the last byte
		// the other way.
		if !slices.Equal(directColour, meshColour) {
			t.Errorf("%s: GPUMeshDirectDepth changes the colour texture", c.name)
		}
		for i := 0; i < len(meshDepth); i += 4 {
			if d := metres(directDepth[i:i+4]) - metres(meshDepth[i:i+4]); math.Abs(d) > 1.5*spread/depthUnits || directDepth[i+3] != meshDepth[i+3] {
				t.Errorf("%s: GPUMeshDirectDepth changes the depth at pixel %d from %v to %v", c.name, i/4, meshDepth[i:i+4], directDepth[i:i+4])
				break
			}
		}
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

// TestGPUClipPlanes checks that the clip planes of GPUMeshDirectDepth give
// a z/w of 0 at the near plane, 1 at the far plane, and
// far/(far-near) * (1-near/d) between them.
func TestGPUClipPlanes(t *testing.T) {
	const near, far = 0.3, 1000
	proj := NewProjectionMatrix4Perspective(60, near, far, 320, 200)
	c := newGPUClipPlanes(proj, near, far)
	for _, d := range []float32{near, 1, 10, 100, far} {
		w := -d * proj[2][3]
		z := c.k * (w - c.near)
		want := float32(far / (far - near) * (1 - near/float64(d)))
		if got := z / w; math.Abs(float64(got-want)) > 1e-6 {
			t.Errorf("at %v m, z/w is %v, want %v", d, got, want)
		}
	}
}

// TestGPUMeshOnePass draws a red cube in front of a green one as models on
// the mesh path, with fog, once with GPUMeshDirectDepth and once with
// GPUMeshOnePass. It checks that the colour textures match, that the
// resolved depth texture matches the depth pass to within 1 mm, with the
// same alpha, and that AfterMeshColour runs once.
func TestGPUMeshOnePass(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() && ebiten.IsDepthSourceSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes or read a depth buffer")
	}
	scene := NewScene("one pass")
	scene.World.LightingOn = false
	scene.World.FogMode = FogOverwrite
	scene.World.FogColor = NewColor4(0.2, 0.4, 0.8, 1)
	scene.World.FogRange = []float32{0, 0.1}
	cube := func(x, z float32, c Color4) *Model {
		m := NewModel("cube", NewCubeMesh(2, 2, 2))
		m.mesh.MeshParts[0].Material.Color = c
		m.mesh.MeshParts[0].Material.Shadeless = true
		m.SetLocalPosition(x, 0, z)
		m.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, 0.4))
		return m
	}
	near := cube(0, -6, NewColor4(1, 0, 0, 1))
	far := cube(1.2, -9, NewColor4(0, 1, 0, 1))
	cam := NewCamera("camera", 64, 64)
	cam.GPUMesh = true
	scene.Root.AddChildren(cam, near, far)
	inFrame(t, func() {
		near.mesh.BuildGPUMesh()
		far.mesh.BuildGPUMesh()
	})
	spread := float64(cam.far-cam.near) * float64(1+2*cam.DepthMargin)
	metres := func(p []byte) float64 {
		return (float64(p[0])*65025 + float64(p[1])*255 + float64(p[2])) / depthUnits * spread
	}

	hooked := 0
	cam.AfterMeshColour = func() { hooked++ }
	read := func(onePass bool) (colour, depth []byte) {
		colour, depth = make([]byte, 4*64*64), make([]byte, 4*64*64)
		cam.GPUMeshDirectDepth, cam.GPUMeshOnePass = !onePass, onePass
		inFrame(t, func() {
			cam.Clear()
			cam.RenderScene(scene)
			cam.ColorTexture().ReadPixels(colour)
			cam.DepthTexture().ReadPixels(depth)
		})
		return
	}
	directColour, directDepth := read(false)
	if hooked != 0 {
		t.Errorf("AfterMeshColour ran %d times without GPUMeshOnePass, want 0", hooked)
	}
	oneColour, oneDepth := read(true)
	if hooked != 1 {
		t.Errorf("AfterMeshColour ran %d times with GPUMeshOnePass, want 1", hooked)
	}

	drawn, colourDiff, depthDiff, maxDiff := 0, 0, 0, 0.0
	for i := 0; i < len(directColour); i += 4 {
		if directDepth[i+3] != 0 {
			drawn++
		}
		for ch := range 4 {
			if d := int(directColour[i+ch]) - int(oneColour[i+ch]); d < -2 || d > 2 {
				colourDiff++
				break
			}
		}
		maxDiff = max(maxDiff, math.Abs(metres(oneDepth[i:i+4])-metres(directDepth[i:i+4])))
		if oneDepth[i+3] != directDepth[i+3] || math.Abs(metres(oneDepth[i:i+4])-metres(directDepth[i:i+4])) > 0.001 {
			depthDiff++
		}
	}
	if drawn < 400 {
		t.Fatalf("the cubes cover %d pixels, want at least 400", drawn)
	}
	if colourDiff > 0 || depthDiff > 0 {
		t.Errorf("%d colour and %d depth pixels differ of %d drawn, want 0, with a depth difference of up to %.4f m", colourDiff, depthDiff, drawn, maxDiff)
	}
}
