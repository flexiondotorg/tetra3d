package tetra3d

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// gpuMeshSource is the Kage vertex function of the mesh path, for a mesh
// that stays on the GPU and is drawn once for each instance record. The mesh
// vertices have the layout of gpuVertices, with the normal of their
// triangle in Custom1, Custom2, and Custom3.
//
// A record places a copy of the mesh with a scale, a yaw about +Y, and a
// translation, and colours it:
//
//	DstX, DstY           the x and z of the origin
//	Custom1              the y of the origin
//	SrcX, SrcY           the cosine and the sine of the yaw
//	Custom0              the scale, negative to mirror along x
//	ColorR, ColorG, ColorB  the tint, and the tinted colour is clamped to 1
//	ColorA               the hue turn, in turns
//
// The hue turn skips a vertex whose alpha is 0 and a colour whose saturation
// is less than 0.45. The colour of a vertex is opaque: the mesh path draws
// only solid parts. GPUVertexMatrix and GPUVertexDepth take the placed
// position to the clip position and the depth, see setGPUUniforms.
// The clip z runs from 0 at the near plane to w at the far plane, so a nearer
// point has a smaller z/w on every graphics library.
//
// GPUMeshCull culls the back faces, as the CPU transform does: it holds the
// camera position in the space of GPUVertexMatrix, and the facing, 1, or -1
// for a model transform that mirrors, or 0 for no culling. A vertex of a back
// face goes past the far plane, and so do the other two, so the GPU draws
// nothing of the triangle.
const gpuMeshSource = `

var GPUVertexMatrix mat4
var GPUVertexDepth vec4
var GPUVertexTint vec4
var GPUVertexTexSize vec2
var GPUMeshCull vec4

func hueTurn(c vec4, turn float) vec3 {
	v := max(max(c.r, c.g), c.b)
	d := v - min(min(c.r, c.g), c.b)
	if turn == 0 || c.a == 0 || v <= 0 || d/v < 0.45 {
		return c.rgb
	}
	h := (c.r-c.g)/d + 4
	if v == c.r {
		h = mod((c.g-c.b)/d, 6)
	} else if v == c.g {
		h = (c.b-c.r)/d + 2
	}
	h = fract(h/6 + turn)
	return v - d + d*clamp(abs(mod(h*6+vec3(0, 4, 2), 6)-3)-1, 0, 1)
}

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iPos vec2, iRot vec2, iTint vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	s := iCustom.x
	a := abs(s)
	local := vec3(dstPos.x*s, dstPos.y*a, custom.x*a)
	world := vec4(local.x*iRot.x+local.z*iRot.y+iPos.x, local.y+iCustom.y, -local.x*iRot.y+local.z*iRot.x+iPos.y, 1)
	p := GPUVertexMatrix * world
	pos := imageDstProjection() * vec4(p.xy+imageDstOrigin()*p.w, p.z, p.w)
	n := vec3(custom.y*sign(s), custom.z, custom.w)
	n = vec3(n.x*iRot.x+n.z*iRot.y, n.y, -n.x*iRot.y+n.z*iRot.x)
	if GPUMeshCull.w*dot(n, GPUMeshCull.xyz-world.xyz) < 0 {
		pos = vec4(0, 0, 2, 1)
	}
	c := vec4(min(hueTurn(color, iTint.a)*iTint.rgb, 1), 1) * GPUVertexTint
	return pos, srcPos*GPUVertexTexSize + imageSrc0Origin(), c, vec4(0, dot(GPUVertexDepth, world), 0, 0)
}
`

// withGPUMesh returns the shader source src with the vertex function of the
// mesh path.
func withGPUMesh(src []byte) []byte {
	return append(append([]byte(nil), src...), gpuMeshSource...)
}

// identityRecord is the instance record that draws a mesh where its model
// places it: the model matrix is in the uniforms.
var identityRecord = []ebiten.Vertex{{SrcX: 1, Custom0: 1, ColorR: 1, ColorG: 1, ColorB: 1}}

// BuildGPUMesh uploads each part of the mesh to the GPU, in the vertex layout
// of gpuVertices, for the mesh path of a camera with GPUMesh on, and of
// Camera.RenderMeshes. Call it at load time, and again after a change to the
// vertices or the triangles of the mesh: until then, the mesh stays with the
// other paths. It panics when ebiten.IsMeshDrawingSupported is false.
func (mesh *Mesh) BuildGPUMesh() {
	if len(mesh.triVertexIndices) != 3*len(mesh.Triangles) {
		mesh.UpdateTriangleData()
	}
	all := mesh.gpuVertices()
	for _, part := range mesh.MeshParts {
		part.gpuMesh = nil
		if part.TriangleCount() == 0 {
			continue
		}
		// Each triangle has vertices of its own, which carry its normal.
		verts := make([]ebiten.Vertex, 0, 3*part.TriangleCount())
		idx := make([]uint32, 0, 3*part.TriangleCount())
		for t := part.TriangleStart; t <= part.TriangleEnd; t++ {
			tv := mesh.triVertexIndices[3*t : 3*t+3]
			p0, p1, p2 := mesh.VertexPositions[tv[0]], mesh.VertexPositions[tv[1]], mesh.VertexPositions[tv[2]]
			n := p1.Sub(p0).Cross(p2.Sub(p0)).Unit()
			for _, v := range tv {
				gv := all[v]
				gv.Custom1, gv.Custom2, gv.Custom3 = n.X, n.Y, n.Z
				idx = append(idx, uint32(len(verts)))
				verts = append(verts, gv)
			}
		}
		part.gpuMesh = ebiten.NewMesh(verts, idx)
		part.gpuMeshVerts = len(mesh.VertexPositions)
	}
}

// gpuVertices returns the vertices of the mesh in the layout of
// the mesh path: the local position in DstX, DstY, and Custom0, the UV in
// SrcX and SrcY with V flipped, and the vertex colour.
func (mesh *Mesh) gpuVertices() []ebiten.Vertex {
	dst := make([]ebiten.Vertex, 0, len(mesh.VertexPositions))
	var colors []Color4
	if mesh.VertexActiveColorChannel >= 0 && mesh.VertexActiveColorChannel < len(mesh.VertexColors) {
		colors = mesh.VertexColors[mesh.VertexActiveColorChannel].colors
	}
	for i, p := range mesh.VertexPositions {
		v := ebiten.Vertex{DstX: p.X, DstY: p.Y, Custom0: p.Z, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
		if i < len(mesh.VertexUVs) {
			uv := mesh.VertexUVs[i]
			v.SrcX, v.SrcY = uv.X, 1-uv.Y
		}
		if i < len(colors) {
			c := colors[i]
			v.ColorR, v.ColorG, v.ColorB, v.ColorA = c.R, c.G, c.B, c.A
		}
		dst = append(dst, v)
	}
	return dst
}

// gpuMeshReady reports whether the part has a GPU mesh that matches the
// vertices of its mesh.
func (part *MeshPart) gpuMeshReady() bool {
	return part.gpuMesh != nil && part.gpuMeshVerts == len(part.Mesh.VertexPositions)
}

// MeshBatch is a mesh that Camera.RenderMeshes draws once for each instance
// record, see gpuMeshSource. Records is in world space.
type MeshBatch struct {
	Mesh    *Mesh
	Records []ebiten.Vertex
}

// meshDraw is one part that the mesh path draws, with the records of its
// MeshBatch.
type meshDraw struct {
	part    *MeshPart
	records []ebiten.Vertex
}

// RenderMeshes draws the solid, unlit meshes of the batches with a hardware
// depth buffer, for each record of each batch, into the colour and the depth
// textures of the camera, before Render draws the other models. Every part of
// each mesh must have a GPU mesh, see Mesh.BuildGPUMesh; the others are left
// out. Call it after Clear and before Render, and once in each frame: the
// depth buffers of the GPU clear only at the first draw of a frame. It does
// not allocate once its buffers have grown.
func (camera *Camera) RenderMeshes(scene *Scene, batches []MeshBatch) {
	if camera.draw == nil {
		camera.draw = newDrawScratch()
	}
	draw := camera.draw
	draw.meshDraws = draw.meshDraws[:0]
	for _, b := range batches {
		for _, part := range b.Mesh.MeshParts {
			if part.gpuMeshReady() && len(b.Records) > 0 {
				draw.meshDraws = append(draw.meshDraws, meshDraw{part: part, records: b.Records})
			}
		}
	}
	camera.drawMeshes(scene, draw.meshDraws)
	clear(draw.meshDraws)
}

// MeshStats returns the draws with a GPU mesh, and the instance records that
// they drew, since the last Clear.
func (camera *Camera) MeshStats() (draws, instances int) {
	return camera.meshDraws, camera.meshInstances
}

// drawMeshes draws the list on the mesh path. The depth pass draws each part
// into depthIntermediate with a hardware depth test, and discards a fragment
// that is behind the depth texture, as the depth pass of the sorted path
// does, so that the parts stay behind the models and the shader draws that
// came before them. The depth texture then takes depthIntermediate, and the
// colour pass draws each part into the colour texture with a hardware depth
// test of its own, where depthIntermediate holds a depth. So each pixel takes
// the nearest part in both passes, with the same depth encoding as the
// sorted path.
func (camera *Camera) drawMeshes(scene *Scene, list []meshDraw) {
	if len(list) == 0 {
		return
	}
	draw := camera.draw
	vp := camera.ViewMatrix().Mult(camera.Projection())
	clip := newGPUClip(camera.Projection(), camera.near, camera.far)
	margin := (camera.far - camera.near) * camera.DepthMargin
	spread := camera.far - camera.near + margin*2
	w, h := camera.resultColorTexture.Bounds().Dx(), camera.resultColorTexture.Bounds().Dy()
	camPos := camera.WorldPosition()
	var world *World
	if scene != nil {
		world = scene.World
	}

	uniforms := func(d *meshDraw) {
		tint, facing := NewColor4(1, 1, 1, 1), float32(1)
		var srcW, srcH float32
		if mat := d.part.Material; mat != nil {
			tint = tint.MultiplyRGBA(mat.Color.ToFloat32s())
			if mat.Texture != nil && mat.UseTexture {
				srcW, srcH = float32(mat.Texture.Bounds().Dx()), float32(mat.Texture.Bounds().Dy())
			}
			if !mat.BackfaceCulling {
				facing = 0
			}
		}
		draw.setGPUUniforms(&vp, clip, w, h, margin, spread, tint, srcW, srcH)
		draw.meshCull[0], draw.meshCull[1], draw.meshCull[2], draw.meshCull[3] = camPos.X, camPos.Y, camPos.Z, facing
	}

	camera.depthIntermediate.Clear()
	opt := &draw.meshDepthOptions
	opt.Images[0] = camera.resultDepthTexture
	for i := range list {
		d := &list[i]
		uniforms(d)
		opt.Mesh = d.part.gpuMesh
		camera.depthIntermediate.DrawTrianglesShader32(d.records, nil, camera.depthShaderMesh, opt)
	}
	camera.resultDepthTexture.DrawImage(camera.depthIntermediate, nil)

	opt = &draw.meshColorOptions
	for i := range list {
		d := &list[i]
		mat := d.part.Material
		img := defaultImg
		filter, fogless := 0, float32(0)
		opt.Blend = ebiten.BlendSourceOver
		if mat != nil {
			if mat.UseTexture && mat.Texture != nil {
				img = mat.Texture
			}
			filter = int(mat.TextureFilterMode)
			if mat.Fogless {
				fogless = 1
			}
			opt.Blend = mat.Blend
		}
		uniforms(d)
		draw.setPartUniforms(0, filter, 0, 1, 1)
		opt.Uniforms = draw.colorUniforms(world, fogless, false)
		opt.Images[0], opt.Images[1] = img, camera.depthIntermediate
		opt.Mesh = d.part.gpuMesh
		camera.resultColorTexture.DrawTrianglesShader32(d.records, nil, camera.colorShaderMesh, opt)

		camera.meshDraws += 2
		camera.meshInstances += len(d.records)
		if camera.DebugInfo.On {
			camera.DebugInfo.drawnParts++
			camera.DebugInfo.drawnTris += d.part.TriangleCount() * len(d.records)
		}
	}
}
