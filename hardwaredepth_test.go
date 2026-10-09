package tetra3d

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestHardwareColourVariant checks that hardwareColour reads no depth image
// and adds the alpha clip, with and without a custom fragment.
func TestHardwareColourVariant(t *testing.T) {
	custom := "package main\n\nfunc CustomFragment(dstPos vec4, srcPos vec2, color vec4) vec4 {\n\treturn color\n}\n"
	for _, c := range []string{"", custom} {
		src := hardwareColour(base3DShaderSource(c))
		if bytes.Contains(src, []byte("imageSrc1")) || !bytes.Contains(src, []byte("var AlphaClip float")) || !bytes.Contains(src, []byte("colorTex.a <= AlphaClip*color.a")) {
			t.Errorf("hardwareColour with custom %q reads a depth image or has no alpha clip", c)
		}
	}
}

// TestHardwareDepth draws a red cube on the mesh path, a green cube on the
// sorted path with a custom depth function, an alpha-clip cube, and a
// transparent blue cube in front of them, once with GPUMeshDirectDepth and
// once with HardwareDepth. It checks that the colour textures match, that
// AfterMeshColour runs once with HardwareDepth, and that HardwareDepth leaves
// the depth texture as Clear left it.
func TestHardwareDepth(t *testing.T) {
	var supported bool
	inFrame(t, func() { supported = ebiten.IsMeshDrawingSupported() && ebiten.IsDepthSourceSupported() })
	if !supported {
		t.Skip("the graphics driver cannot draw meshes or read a depth buffer")
	}
	scene := NewScene("hardware depth")
	scene.World.LightingOn = false
	scene.World.FogOn = false
	cube := func(x, y, z float32, c Color4) *Model {
		m := NewModel("cube", NewCubeMesh(2, 2, 2))
		m.mesh.MeshParts[0].Material.Color = c
		m.mesh.MeshParts[0].Material.Shadeless = true
		m.SetLocalPosition(x, y, z)
		m.SetLocalRotation(NewMatrix4Rotate(0, 1, 0, 0.4))
		return m
	}
	onMesh := cube(0, 0, -6, NewColor4(1, 0, 0, 1))
	sorted := cube(1.2, 0, -9, NewColor4(0, 1, 0, 1))
	sorted.mesh.MeshParts[0].Material.CustomDepthFunction = func(_ *Model, _ *Camera, _ *MeshPart, _ int, depth float32) float32 { return depth }
	clipped := cube(-1.5, 0.5, -8, NewColor4(1, 1, 0, 1))
	glass := cube(0.5, -0.4, -4.5, NewColor4(0, 0, 1, 0.5))
	glass.SetLocalScale(0.4, 0.4, 0.4)
	glass.mesh.MeshParts[0].Material.TransparencyMode = TransparencyModeTransparent
	cam := NewCamera("camera", 64, 64)
	cam.GPUMesh = true
	scene.Root.AddChildren(cam, onMesh, sorted, clipped, glass)
	inFrame(t, func() {
		onMesh.mesh.BuildGPUMesh()
		// A checker of opaque and clear texels for the alpha clip.
		tex := ebiten.NewImage(2, 2)
		tex.Set(0, 0, color.White)
		tex.Set(1, 1, color.White)
		mat := clipped.mesh.MeshParts[0].Material
		mat.Texture, mat.UseTexture = tex, true
		mat.TransparencyMode = TransparencyModeAlphaClip
	})

	hooked := 0
	cam.AfterMeshColour = func() { hooked++ }
	read := func(hardware bool) (colour, depth []byte) {
		colour, depth = make([]byte, 4*64*64), make([]byte, 4*64*64)
		cam.GPUMeshDirectDepth, cam.HardwareDepth = !hardware, hardware
		inFrame(t, func() {
			cam.Clear()
			cam.RenderScene(scene)
			cam.ColorTexture().ReadPixels(colour)
			cam.DepthTexture().ReadPixels(depth)
		})
		return
	}
	directColour, _ := read(false)
	if hooked != 0 {
		t.Errorf("AfterMeshColour ran %d times without HardwareDepth, want 0", hooked)
	}
	hardColour, hardDepth := read(true)
	if hooked != 1 {
		t.Errorf("AfterMeshColour ran %d times with HardwareDepth, want 1", hooked)
	}
	if !bytes.Equal(hardDepth, make([]byte, len(hardDepth))) {
		t.Errorf("HardwareDepth wrote into the depth texture")
	}

	drawn, colourDiff := 0, 0
	var seen [4]int // Pixels where red, green, yellow, and the blue of the glass show.
	for i := 0; i < len(directColour); i += 4 {
		p := directColour[i : i+4]
		if bytes.Equal(p, directColour[:4]) && bytes.Equal(hardColour[i:i+4], hardColour[:4]) {
			continue
		}
		drawn++
		switch {
		case p[2] > 60 && p[2] > p[0] && p[2] > p[1]:
			seen[3]++
		case p[0] > 200 && p[1] > 200:
			seen[2]++
		case p[0] > 200:
			seen[0]++
		case p[1] > 200:
			seen[1]++
		}
		for ch := range 4 {
			if d := int(p[ch]) - int(hardColour[i+ch]); d < -2 || d > 2 {
				colourDiff++
				break
			}
		}
	}
	for k, n := range seen {
		if n < 20 {
			t.Fatalf("colour %d covers %d pixels, want at least 20 (%v)", k, n, seen)
		}
	}
	if colourDiff > drawn/50 {
		t.Errorf("%d colour pixels differ of %d drawn, want at most %d", colourDiff, drawn, drawn/50)
	}
}
