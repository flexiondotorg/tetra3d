package tetra3d

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestSetDepthTexture gives the camera a depth texture of its own that holds
// a depth near the camera, with an alpha that is neither 0 nor 1, and checks
// that Clear leaves it, that it hides a red cube, and that the cube shows
// again with the camera's own depth texture.
func TestSetDepthTexture(t *testing.T) {
	scene := NewScene("depth")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := NewModel("cube", NewCubeMesh(2, 2, 2))
	cube.mesh.MeshParts[0].Material.Color = NewColor4(1, 0, 0, 1)
	cube.SetLocalPosition(0, 0, -6)
	cam := NewCamera("camera", 64, 64)
	scene.Root.AddChildren(cam, cube)
	near := make([]byte, 64*64*4)
	for p := 0; p < len(near); p += 4 {
		near[p+2], near[p+3] = 1, 3
	}
	var depth *ebiten.Image
	inFrame(t, func() {
		depth = ebiten.NewImage(64, 64)
		depth.WritePixels(near)
	})
	for _, c := range []struct {
		name string
		own  bool
		red  bool
	}{{"caller", false, false}, {"own", true, true}} {
		var pix [4]byte
		kept := make([]byte, len(near))
		inFrame(t, func() {
			if c.own {
				cam.SetDepthTexture(nil)
			} else {
				cam.SetDepthTexture(depth)
			}
			cam.Clear()
			depth.ReadPixels(kept)
			cam.RenderScene(scene)
			var all [64 * 64 * 4]byte
			cam.ColorTexture().ReadPixels(all[:])
			pix = [4]byte(all[4*(32*64+32):])
		})
		if !c.own && string(kept) != string(near) {
			t.Errorf("%s: Clear changed the depth texture of the caller", c.name)
		}
		if red := pix == [4]byte{255, 0, 0, 255}; red != c.red {
			t.Errorf("%s: pixel %v, want red %v", c.name, pix, c.red)
		}
	}
}
