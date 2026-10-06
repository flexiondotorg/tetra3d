package tetra3d

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestGPUMeshNearestColour draws a red cube in front of a green one with the
// sorted path, with both on the mesh path, and with one on each path, and
// checks that the pixel where they overlap is red, and a pixel of the green
// cube alone is green, each time.
func TestGPUMeshNearestColour(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("cubes")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := func(x, z float32, c Color4) *Model {
		m := NewModel("cube", NewCubeMesh(2, 2, 2))
		m.mesh.MeshParts[0].Material.Color = c
		m.mesh.MeshParts[0].Material.Shadeless = true
		m.SetLocalPosition(x, 0, z)
		return m
	}
	near := cube(0, -6, NewColor4(1, 0, 0, 1))
	far := cube(1.2, -9, NewColor4(0, 1, 0, 1))
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam, near, far)
	inFrame(t, func() {
		near.mesh.BuildGPUMesh()
		far.mesh.BuildGPUMesh()
	})
	nearMesh, farMesh := near.mesh.MeshParts[0].gpuMesh, far.mesh.MeshParts[0].gpuMesh

	for _, c := range []struct {
		name      string
		on        bool
		near, far *ebiten.Mesh
		draws     int
	}{
		{"sorted", false, nearMesh, farMesh, 0},
		{"mesh", true, nearMesh, farMesh, 4},
		{"near on the mesh path", true, nearMesh, nil, 2},
		{"far on the mesh path", true, nil, farMesh, 2},
	} {
		var overlap, farOnly color.Color
		var draws int
		inFrame(t, func() {
			cam.GPUMesh = c.on
			near.mesh.MeshParts[0].gpuMesh, far.mesh.MeshParts[0].gpuMesh = c.near, c.far
			cam.Clear()
			cam.RenderScene(scene)
			overlap, farOnly = cam.ColorTexture().At(38, 32), cam.ColorTexture().At(45, 32)
			draws, _ = cam.MeshStats()
		})
		if r, g, b, a := overlap.RGBA(); r != 0xffff || g != 0 || b != 0 || a != 0xffff {
			t.Errorf("%s: overlap %v, want red", c.name, overlap)
		}
		if r, g, b, a := farOnly.RGBA(); r != 0 || g != 0xffff || b != 0 || a != 0xffff {
			t.Errorf("%s: far cube %v, want green", c.name, farOnly)
		}
		if draws != c.draws {
			t.Errorf("%s: %d mesh draws, want %d", c.name, draws, c.draws)
		}
	}
}

// TestQueueMeshes draws a green cube from a MeshBatch and a red model on the
// mesh path, each in front of the other in turn, with RenderMeshes and with
// QueueMeshes, and checks that the images are the same and that the nearer
// cube covers the overlap.
func TestQueueMeshes(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("queue")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	green := NewCubeMesh(2, 2, 2)
	green.MeshParts[0].Material.Color = NewColor4(0, 1, 0, 1)
	red := NewModel("red", NewCubeMesh(2, 2, 2))
	red.mesh.MeshParts[0].Material.Color = NewColor4(1, 0, 0, 1)
	cam := NewCamera("camera", 64, 64)
	cam.GPUMesh = true
	scene.Root.AddChildren(cam, red)
	inFrame(t, func() {
		green.BuildGPUMesh()
		red.mesh.BuildGPUMesh()
	})
	for _, c := range []struct {
		redZ, greenZ float32
		want         [3]byte
	}{{-6, -9, [3]byte{255, 0, 0}}, {-9, -6, [3]byte{0, 255, 0}}} {
		red.SetLocalPosition(0, 0, c.redZ)
		rec := ebiten.Vertex{DstX: 1.2, DstY: c.greenZ, SrcX: 1, ColorR: 1, ColorG: 1, ColorB: 1, Custom0: 1}
		batches := []MeshBatch{{Mesh: green, Records: []ebiten.Vertex{rec}}}
		var images [2][64 * 64 * 4]byte
		for i, queue := range []bool{false, true} {
			inFrame(t, func() {
				cam.Clear()
				if queue {
					cam.QueueMeshes(batches)
				} else {
					cam.RenderMeshes(scene, batches)
				}
				cam.RenderScene(scene)
				cam.ColorTexture().ReadPixels(images[i][:])
			})
		}
		if images[0] != images[1] {
			t.Errorf("red at z %v: the images of RenderMeshes and QueueMeshes differ", c.redZ)
		}
		p := 4 * (32*64 + 38)
		if got := [3]byte(images[1][p : p+3]); got != c.want {
			t.Errorf("red at z %v: overlap %v, want %v", c.redZ, got, c.want)
		}
	}
}

// TestRigidRecords draws two models that share a cube mesh with a colour on
// each face, one turned and scaled unevenly and one mirrored, with the sorted
// path and with the mesh path, and checks that the mesh path draws them as
// one part with two records, and that the images match but for edge pixels.
func TestRigidRecords(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("cubes")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := NewCubeMesh(2, 2, 2)
	cube.ensureEnoughVertexColorChannels(0)
	cube.VertexActiveColorChannel = 0
	faces := [6]Color4{{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}, {1, 1, 0, 1}, {0, 1, 1, 1}, {1, 0, 1, 1}}
	for i := range cube.VertexPositions {
		cube.VertexColors[0].colors[i] = faces[i/4]
	}
	a, b := NewModel("a", cube), NewModel("b", cube)
	a.SetLocalPosition(-1.4, 0, -7)
	a.SetLocalRotation(NewMatrix4Rotate(1, 1, 0, 0.7))
	a.SetLocalScale(1, 0.6, 1.4)
	b.SetLocalPosition(1.5, 0.3, -8)
	b.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, 0.4))
	b.SetLocalScale(-1, 1, 1)
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam, a, b)
	inFrame(t, func() { cube.BuildGPUMesh() })

	var images [2][64 * 64 * 4]byte
	var draws, records int
	for i, on := range []bool{false, true} {
		inFrame(t, func() {
			cam.GPUMesh = on
			cam.Clear()
			cam.RenderScene(scene)
			cam.ColorTexture().ReadPixels(images[i][:])
			draws, records = cam.MeshStats()
		})
	}
	if draws != 2 || records != 2 {
		t.Errorf("%d mesh draws and %d records, want 2 and 2", draws, records)
	}
	drawn, differ := 0, 0
	for p := 0; p < len(images[0]); p += 4 {
		if [4]byte(images[0][p:p+4]) != [4]byte(images[0][:4]) {
			drawn++
		}
		for c := range 4 {
			if d := int(images[0][p+c]) - int(images[1][p+c]); d > 8 || d < -8 {
				differ++
				break
			}
		}
	}
	if drawn < 200 || differ > drawn/20 {
		t.Errorf("%d of %d drawn pixels differ", differ, drawn)
	}
}

// TestPackGPUNormal checks that the decode of gpuPoseSource, repeated in Go,
// gives back each packed normal to within the 12 bits of each value.
func TestPackGPUNormal(t *testing.T) {
	unpack := func(v float32) Vector3 {
		hi := float32(math.Floor(float64(v / 4096)))
		x, y := (v-hi*4096)/4095*2-1, hi/4095*2-1
		n := Vector3{X: x, Y: y, Z: 1 - abs32(x) - abs32(y)}
		if n.Z < 0 {
			n.X, n.Y = (1-abs32(y))*signNotZero(x), (1-abs32(x))*signNotZero(y)
		}
		return n.Unit()
	}
	for _, n := range []Vector3{{1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}, {0.3, -0.5, -0.8}, {-0.7, 0.1, 0.2}} {
		n = n.Unit()
		p := PackGPUNormal(n)
		if p != float32(int32(p)) || p < 0 || p >= 1<<24 {
			t.Errorf("%v packs to %v, not an integer below 2^24", n, p)
		}
		if d := unpack(p).Sub(n).Magnitude(); d > 2e-3 {
			t.Errorf("%v unpacks %v away", n, d)
		}
	}
}

func abs32(v float32) float32 { return float32(math.Abs(float64(v))) }

// TestGPUPose draws a cube on the mesh path as a MeshBatch with Pose, whose
// vertices have an offset of 3 along +x to the peak pose, and checks that the
// cube stands at its rest position with a factor of 0 and at the offset with
// a factor of 1.
func TestGPUPose(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("pose")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := NewCubeMesh(2, 2, 2)
	cube.MeshParts[0].Material.Color = NewColor4(1, 0, 0, 1)
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam)
	inFrame(t, func() {
		cube.BuildGPUMeshFunc(func(_, _ int, v *ebiten.Vertex) {
			n := PackGPUNormal(Vector3{X: v.Custom1, Y: v.Custom2, Z: v.Custom3})
			v.SrcX, v.SrcY, v.Custom1, v.Custom2, v.Custom3 = 1.5, 0, 0, n, n
		})
	})
	for _, c := range []struct {
		k               float32
		filled, cleared int // The x of a pixel that the cube covers, and of one that it does not.
	}{{0, 32, 50}, {1, 50, 32}} {
		var filled, cleared color.Color
		inFrame(t, func() {
			cam.Clear()
			rec := ebiten.Vertex{DstY: -8, SrcX: 1, ColorR: 1, ColorG: 1, ColorB: 1, Custom0: 2, Custom2: c.k}
			cam.RenderMeshes(scene, []MeshBatch{{Mesh: cube, Records: []ebiten.Vertex{rec}, Pose: true}})
			filled, cleared = cam.ColorTexture().At(c.filled, 32), cam.ColorTexture().At(c.cleared, 32)
		})
		if r, _, _, a := filled.RGBA(); r != 0xffff || a != 0xffff {
			t.Errorf("k %v: pixel %d is %v, want red", c.k, c.filled, filled)
		}
		if r, _, _, _ := cleared.RGBA(); r == 0xffff {
			t.Errorf("k %v: pixel %d is %v, want the clear colour", c.k, c.cleared, cleared)
		}
	}
}

// TestGPUBend draws a bar that bends about two joints with a stretch, with
// the sorted path, where the processor writes the bent vertices, and with
// the mesh path and a Bend, and checks that the images match but for edge
// pixels.
func TestGPUBend(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("bend")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	bar := NewCubeMesh(4, 1, 1)
	bar.MeshParts[0].Material.Color = NewColor4(1, 0.5, 0, 1)
	// The left end moves by half of first, and the right end by second and
	// then first.
	value := func(p Vector3) float32 {
		if p.X > 0 {
			return 3
		}
		return 0.5
	}
	b := Bend{
		First:   NewMatrix4Translate(1, 0, 0).Mult(NewMatrix4Rotate(0, 0, 1, 0.5)).Mult(NewMatrix4Translate(-1, 0, 0)),
		Second:  NewMatrix4Rotate(1, 0, 0, 0.8).Mult(NewMatrix4Translate(0, 0.4, 0)),
		Stretch: 1.17,
		Along:   Vector3{X: 1},
	}
	bent := bar.Clone()
	for i, p := range bent.VertexPositions {
		v := value(p)
		if v >= 2 {
			q := b.Second.MultVec(p)
			p = b.First.MultVec(p.Add(q.Sub(p).Scale(v - 2)))
		} else {
			p = p.Add(b.First.MultVec(p).Sub(p).Scale(v))
		}
		bent.VertexPositions[i] = p.Add(b.Along.Scale((b.Stretch - 1) * p.Dot(b.Along)))
	}
	gpu, cpu := NewModel("gpu", bar), NewModel("cpu", bent)
	for _, m := range []*Model{gpu, cpu} {
		m.SetLocalPosition(0, -0.5, -9)
		m.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, 0.6))
	}
	gpu.GPUBend = &b
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam, gpu, cpu)
	inFrame(t, func() {
		bar.BuildGPUMeshFunc(func(_, v int, gv *ebiten.Vertex) { gv.ColorA = value(bar.VertexPositions[v]) })
	})

	var images [2][64 * 64 * 4]byte
	var draws int
	for i, on := range []bool{false, true} {
		inFrame(t, func() {
			cam.GPUMesh = on
			gpu.SetVisible(on, false)
			cpu.SetVisible(!on, false)
			cam.Clear()
			cam.RenderScene(scene)
			cam.ColorTexture().ReadPixels(images[i][:])
			draws, _ = cam.MeshStats()
		})
	}
	if draws != 2 {
		t.Errorf("%d mesh draws, want 2", draws)
	}
	drawn, differ := 0, 0
	for p := 0; p < len(images[0]); p += 4 {
		if [4]byte(images[0][p:p+4]) != [4]byte(images[0][:4]) {
			drawn++
		}
		for c := range 4 {
			if d := int(images[0][p+c]) - int(images[1][p+c]); d > 8 || d < -8 {
				differ++
				break
			}
		}
	}
	if drawn < 200 || differ > drawn/20 {
		t.Errorf("%d of %d drawn pixels differ", differ, drawn)
	}
}

// TestClearForMeshes draws a red cube in front of a green one, and a wide
// blue box just inside the far plane, on the mesh path, over a
// depthIntermediate that holds stale values. It checks that the red cube
// shows where the cubes overlap and that the blue box shows in a corner, so
// the quad at the far plane hides nothing. In the next frame it writes stale
// values again, clears them with clearForMeshes, and checks that every byte
// is zero.
func TestClearForMeshes(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes")
	}
	scene := NewScene("clear")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := func(x, z float32, c Color4) *Model {
		m := NewModel("cube", NewCubeMesh(2, 2, 2))
		m.mesh.MeshParts[0].Material.Color = c
		m.mesh.MeshParts[0].Material.Shadeless = true
		m.SetLocalPosition(x, 0, z)
		return m
	}
	near := cube(0, -6, NewColor4(1, 0, 0, 1))
	far := cube(1.2, -9, NewColor4(0, 1, 0, 1))
	cam := NewCamera("camera", 64, 64)
	cam.GPUMesh = true
	back := cube(0, -cam.Far()+1.5, NewColor4(0, 0, 1, 1))
	back.SetLocalScale(80, 80, 1)
	scene.Root.AddChildren(cam, near, far, back)
	inFrame(t, func() {
		near.mesh.BuildGPUMesh()
		far.mesh.BuildGPUMesh()
		back.mesh.BuildGPUMesh()
	})
	stale := make([]byte, 64*64*4)
	for p := range stale {
		stale[p] = byte(p*37 + 11)
	}
	var cleared, colour [64 * 64 * 4]byte
	inFrame(t, func() {
		cam.Clear()
		cam.depthIntermediate.WritePixels(stale)
		cam.RenderScene(scene)
		cam.ColorTexture().ReadPixels(colour[:])
	})
	inFrame(t, func() {
		cam.depthIntermediate.WritePixels(stale)
		cam.clearForMeshes(cam.depthIntermediate)
		cam.depthIntermediate.ReadPixels(cleared[:])
	})
	if cleared != [64 * 64 * 4]byte{} {
		t.Errorf("clearForMeshes left stale values in depthIntermediate")
	}
	if p := 4 * (32*64 + 32); [3]byte(colour[p:p+3]) != [3]byte{255, 0, 0} {
		t.Errorf("the overlap is %v, want red", colour[p:p+3])
	}
	if p := 4 * (2*64 + 2); [3]byte(colour[p:p+3]) != [3]byte{0, 0, 255} {
		t.Errorf("the far box is %v in a corner, want blue", colour[p:p+3])
	}
}
