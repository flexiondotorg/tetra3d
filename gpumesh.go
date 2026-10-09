package tetra3d

import (
	"math"
	"slices"

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
const gpuMeshSource = gpuMeshHeader + `
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

// gpuMeshHeader holds the uniforms and the hue turn of gpuMeshSource and
// gpuPoseSource.
const gpuMeshHeader = `

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
`

// gpuPoseSource is the vertex function of gpuMeshSource for a mesh that
// moves between a rest pose and a peak pose, see MeshBatch.Pose. A vertex has
// its rest position in DstX, DstY, and Custom0, the offset to its peak
// position in SrcX, SrcY, and Custom1, and the normal of its triangle at rest
// and at the peak in Custom2 and Custom3, each packed with PackGPUNormal. The
// record is that of gpuMeshSource, with the pose factor k in Custom2: the
// vertex moves to its rest position plus k times its offset, and the back-face
// test reads the mix of the two normals by k. The mesh has no texture.
const gpuPoseSource = gpuMeshHeader + `
func unpackNormal(v float) vec3 {
	hi := floor(v / 4096)
	e := vec2(v-hi*4096, hi)/4095*2 - 1
	n := vec3(e, 1-abs(e.x)-abs(e.y))
	if n.z < 0 {
		n.xy = (1 - abs(n.yx)) * (step(0, n.xy)*2 - 1)
	}
	return n
}

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iPos vec2, iRot vec2, iTint vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	s := iCustom.x
	a := abs(s)
	k := iCustom.z
	q := vec3(dstPos, custom.x) + k*vec3(srcPos, custom.y)
	local := vec3(q.x*s, q.y*a, q.z*a)
	world := vec4(local.x*iRot.x+local.z*iRot.y+iPos.x, local.y+iCustom.y, -local.x*iRot.y+local.z*iRot.x+iPos.y, 1)
	p := GPUVertexMatrix * world
	pos := imageDstProjection() * vec4(p.xy+imageDstOrigin()*p.w, p.z, p.w)
	n := mix(unpackNormal(custom.z), unpackNormal(custom.w), k)
	n.x *= sign(s)
	n = vec3(n.x*iRot.x+n.z*iRot.y, n.y, -n.x*iRot.y+n.z*iRot.x)
	if GPUMeshCull.w*dot(n, GPUMeshCull.xyz-world.xyz) < 0 {
		pos = vec4(0, 0, 2, 1)
	}
	c := vec4(min(hueTurn(color, iTint.a)*iTint.rgb, 1), 1) * GPUVertexTint
	return pos, imageSrc0Origin(), c, vec4(0, dot(GPUVertexDepth, world), 0, 0)
}
`

// PackGPUNormal packs the unit vector n into one float for gpuPoseSource: the
// octahedral map of n, with 12 bits for each of its two values. The result is
// an integer below 2^24, so a float32 holds it exactly.
func PackGPUNormal(n Vector3) float32 {
	s := float32(math.Abs(float64(n.X)) + math.Abs(float64(n.Y)) + math.Abs(float64(n.Z)))
	if s == 0 {
		return 0
	}
	x, y := n.X/s, n.Y/s
	if n.Z < 0 {
		x, y = (1-float32(math.Abs(float64(y))))*signNotZero(x), (1-float32(math.Abs(float64(x))))*signNotZero(y)
	}
	q := func(v float32) float32 { return float32(math.Round(float64((v*0.5 + 0.5) * 4095))) }
	return q(x) + 4096*q(y)
}

// signNotZero returns 1 for v >= 0, and -1 otherwise.
func signNotZero(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// gpuRigidSource is the Kage vertex function of the mesh path for models,
// which Camera.Render draws with one record for each model that shows a mesh
// part, see rigidRecord. A record places the mesh with the world transform of
// its model: a model-space point p goes to p.x*r0 + p.y*r1 + p.z*r2 + t, with
//
//	ColorR, ColorG, ColorB  r0
//	Custom0, Custom1, Custom2  r1
//	SrcY                 f, so that r2 is f times the cross product of r0 and r1
//	DstX, DstY, SrcX     t
//	ColorA               the alpha of the model colour
//	Custom3              a depth bias, which moves the mesh towards the camera
//	                     along the view ray, in world units
//
// That holds every transform of a scale on the axes of the model, a rotation,
// and a translation, and a mirror as a negative f. The uniforms are those of
// gpuMeshSource, with the colour of the model and the material in
// GPUVertexTint.
const gpuRigidSource = `

var GPUVertexMatrix mat4
var GPUVertexDepth vec4
var GPUVertexTint vec4
var GPUVertexTexSize vec2
var GPUMeshCull vec4

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iPos vec2, iRot vec2, iTint vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	r0 := iTint.xyz
	r1 := iCustom.xyz
	r2 := iRot.y * cross(r0, r1)
	world := vec4(dstPos.x*r0+dstPos.y*r1+custom.x*r2+vec3(iPos, iRot.x), 1)
	// The cofactors turn the normal, and turn it over for a mirror.
	n := custom.y*cross(r1, r2) + custom.z*cross(r2, r0) + custom.w*cross(r0, r1)
	back := GPUMeshCull.w*dot(n, GPUMeshCull.xyz-world.xyz) < 0
	view := GPUMeshCull.xyz - world.xyz
	world.xyz += iCustom.w * view / max(length(view), 0.001)
	p := GPUVertexMatrix * world
	pos := imageDstProjection() * vec4(p.xy+imageDstOrigin()*p.w, p.z, p.w)
	if back {
		pos = vec4(0, 0, 2, 1)
	}
	return pos, srcPos*GPUVertexTexSize + imageSrc0Origin(), color * GPUVertexTint * vec4(1, 1, 1, iTint.a), vec4(0, dot(GPUVertexDepth, world), 0, 0)
}
`

// gpuBendSource is the vertex function of gpuRigidSource for a model with a
// Bend: it bends each point of the mesh about two joints before the record
// places it. The colour alpha of a vertex holds its joint value, see Bend,
// and the vertex draws opaque. GPUBendFirst and GPUBendSecond are the two
// joint transforms of the Bend, and GPUBendStretch holds its direction and
// its stretch minus 1.
const gpuBendSource = `

var GPUVertexMatrix mat4
var GPUVertexDepth vec4
var GPUVertexTint vec4
var GPUVertexTexSize vec2
var GPUMeshCull vec4
var GPUBendFirst mat4
var GPUBendSecond mat4
var GPUBendStretch vec4

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iPos vec2, iRot vec2, iTint vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	v := color.a
	b := vec3(dstPos, custom.x)
	n := custom.yzw
	if v >= 2 {
		b = mix(b, (GPUBendSecond * vec4(b, 1)).xyz, v-2)
		n = mix(n, (GPUBendSecond * vec4(n, 0)).xyz, v-2)
		b = (GPUBendFirst * vec4(b, 1)).xyz
		n = (GPUBendFirst * vec4(n, 0)).xyz
	} else {
		b = mix(b, (GPUBendFirst * vec4(b, 1)).xyz, v)
		n = mix(n, (GPUBendFirst * vec4(n, 0)).xyz, v)
	}
	b += GPUBendStretch.xyz * (GPUBendStretch.w * dot(b, GPUBendStretch.xyz))
	r0 := iTint.xyz
	r1 := iCustom.xyz
	r2 := iRot.y * cross(r0, r1)
	world := vec4(b.x*r0+b.y*r1+b.z*r2+vec3(iPos, iRot.x), 1)
	n = n.x*cross(r1, r2) + n.y*cross(r2, r0) + n.z*cross(r0, r1)
	back := GPUMeshCull.w*dot(n, GPUMeshCull.xyz-world.xyz) < 0
	view := GPUMeshCull.xyz - world.xyz
	world.xyz += iCustom.w * view / max(length(view), 0.001)
	p := GPUVertexMatrix * world
	pos := imageDstProjection() * vec4(p.xy+imageDstOrigin()*p.w, p.z, p.w)
	if back {
		pos = vec4(0, 0, 2, 1)
	}
	return pos, srcPos*GPUVertexTexSize + imageSrc0Origin(), vec4(color.rgb, 1) * GPUVertexTint * vec4(1, 1, 1, iTint.a), vec4(0, dot(GPUVertexDepth, world), 0, 0)
}
`

// Bend bends the mesh of a model on the mesh path about two joints, for a
// model with a GPU mesh whose colour alpha holds the joint value of each
// vertex, see Mesh.BuildGPUMeshFunc. A vertex with the value w from 0 to 1
// moves by the share w of the turn First. A vertex with the value 2 + w
// moves by the share w of the turn Second, and then by the whole of First.
// The transforms apply to a point as Matrix4.MultVec does. Then a Stretch
// s > 1 lengthens the mesh from its origin along the unit direction Along:
// a point p moves by Along times (s - 1) times the dot product of p and
// Along. The value of a vertex replaces its colour alpha, so the vertex
// draws opaque.
type Bend struct {
	First, Second Matrix4
	Stretch       float32
	Along         Vector3
}

// withGPUMesh returns the shader source src with the vertex function of the
// mesh path.
func withGPUMesh(src []byte) []byte {
	return append(append([]byte(nil), src...), gpuMeshSource...)
}

// withGPUPose returns the shader source src with the vertex function of the
// mesh path for a mesh that moves between two poses.
func withGPUPose(src []byte) []byte {
	return append(append([]byte(nil), src...), gpuPoseSource...)
}

// withGPURigid returns the shader source src with the vertex function of the
// mesh path for models.
func withGPURigid(src []byte) []byte {
	return append(append([]byte(nil), src...), gpuRigidSource...)
}

// withGPUBend returns the shader source src with the vertex function of the
// mesh path for models with a Bend.
func withGPUBend(src []byte) []byte {
	return append(append([]byte(nil), src...), gpuBendSource...)
}

// rigidShaders holds the variant for the mesh path of each shader that
// ExtendBase3DShader makes, so that a material with such a shader can draw
// on the mesh path.
var rigidShaders = map[*ebiten.Shader]*ebiten.Shader{}

// bendShaders holds the variant for models with a Bend of each shader that
// ExtendBase3DShader makes.
var bendShaders = map[*ebiten.Shader]*ebiten.Shader{}

// onePassRigidShaders and onePassBendShaders hold the variants of
// rigidShaders and bendShaders for Camera.GPUMeshOnePass.
var onePassRigidShaders, onePassBendShaders = map[*ebiten.Shader]*ebiten.Shader{}, map[*ebiten.Shader]*ebiten.Shader{}

// rigidRecord returns the instance record of gpuRigidSource for model, with
// a depth bias.
func rigidRecord(model *Model, bias float32) ebiten.Vertex {
	t := model.Transform()
	r0, r1, r2 := t.RowAsVector3(0), t.RowAsVector3(1), t.RowAsVector3(2)
	c := r0.Cross(r1)
	f := float32(0)
	if d := c.Dot(c); d > 0 {
		f = r2.Dot(c) / d
	}
	return ebiten.Vertex{
		DstX: t[3][0], DstY: t[3][1], SrcX: t[3][2], SrcY: f,
		ColorR: r0.X, ColorG: r0.Y, ColorB: r0.Z, ColorA: model.Color.A,
		Custom0: r1.X, Custom1: r1.Y, Custom2: r1.Z, Custom3: bias,
	}
}

// BuildGPUMesh uploads each part of the mesh to the GPU, in the vertex layout
// of gpuVertices, for the mesh path of a camera with GPUMesh on, and of
// Camera.RenderMeshes. Call it at load time, and again after a change to the
// vertices or the triangles of the mesh: until then, the mesh stays with the
// other paths. It panics when ebiten.IsMeshDrawingSupported is false.
func (mesh *Mesh) BuildGPUMesh() {
	mesh.BuildGPUMeshFunc(nil)
}

// BuildGPUMeshFunc is BuildGPUMesh, and it calls fill, when not nil, for each
// corner of each triangle, with the index of the triangle and of its vertex in
// the mesh, and the GPU vertex, which fill can change. The GPU vertex holds
// the normal of the triangle in Custom1, Custom2, and Custom3. For example,
// fill writes the pose data of gpuPoseSource.
func (mesh *Mesh) BuildGPUMeshFunc(fill func(tri, vertex int, v *ebiten.Vertex)) {
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
				if fill != nil {
					fill(t, int(v), &gv)
				}
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

// gpuPlaceable reports whether a vertex shader can place part of model for
// the camera: nothing moves the vertices or needs them on the processor.
func (camera *Camera) gpuPlaceable(model *Model, part *MeshPart, lighting bool) bool {
	if !camera.perspective || !camera.RenderDepth || camera.RenderNormals ||
		camera.PerspectiveCorrectedTextureMapping || camera.VertexSnapping > 0 || lighting {
		return false
	}
	mesh := part.Mesh
	if model.DynamicBatchOwner != nil || model.skinned || model.VertexTransformFunction != nil || model.VertexClipFunction != nil ||
		mesh.autoSubdivide || len(mesh.shapeKeys) > 0 {
		return false
	}
	mat := part.Material
	return mat == nil || (!mat.BillboardEnabled && mat.BillboardedDepthMode != DepthModeUnbillboarded && mat.TextureMapMode == 0)
}

// gpuMeshReady reports whether the part has a GPU mesh that matches the
// vertices of its mesh.
func (part *MeshPart) gpuMeshReady() bool {
	return part.gpuMesh != nil && part.gpuMeshVerts == len(part.Mesh.VertexPositions)
}

// gpuMeshPart reports whether Camera.Render draws part of model on the mesh
// path: the camera has GPUMesh on, the part has a current GPU mesh, a vertex
// shader can place it, it is opaque with no alpha clip, and its fragment
// shader, if any, has a variant in rigidShaders. Alpha-clip and transparent
// parts need the sorted path. On the mesh path, a custom depth function
// it becomes a depth bias, see groupRigid.
func (camera *Camera) gpuMeshPart(model *Model, part *MeshPart, lighting bool) bool {
	if !camera.GPUMesh || !part.gpuMeshReady() || !camera.gpuPlaceable(model, part, lighting) || model.isTransparent(part) {
		return false
	}
	mat := part.Material
	variants := camera.rigidVariants(model)
	return mat == nil || mat.TransparencyMode != TransparencyModeAlphaClip && (!mat.shaderOn() || variants[mat.fragmentShader] != nil)
}

// rigidVariants returns the variants of the fragment shaders of materials
// that the colour pass of the mesh path draws model with.
func (camera *Camera) rigidVariants(model *Model) map[*ebiten.Shader]*ebiten.Shader {
	onePass := camera.GPUMeshOnePass && camera.perspective
	switch {
	case model.GPUBend != nil && onePass:
		return onePassBendShaders
	case model.GPUBend != nil:
		return bendShaders
	case onePass:
		return onePassRigidShaders
	}
	return rigidShaders
}

// MeshBatch is a mesh that Camera.RenderMeshes draws once for each instance
// record, see gpuMeshSource. Records is in world space. With Pose, the mesh
// moves between two poses, see gpuPoseSource and Mesh.BuildGPUMeshFunc.
type MeshBatch struct {
	Mesh    *Mesh
	Records []ebiten.Vertex
	Pose    bool
}

// meshDraw is one part that the mesh path draws: with the records of a
// MeshBatch when model is nil, or with the records of gpuRigidSource for the
// models that show it in the colour of model.
type meshDraw struct {
	part    *MeshPart
	model   *Model
	records []ebiten.Vertex
	n       int  // The number of records, while groupRigid counts them.
	pose    bool // The records are those of gpuPoseSource.
}

// rigidPart is a part of a model that Camera.Render draws on the mesh path.
type rigidPart struct {
	part  *MeshPart
	model *Model
	group int // The index of its meshDraw.
}

// groupRigid appends to s.meshDraws one draw for each mesh part in s.rigid,
// and for each colour of the models that show it, with a record for each of
// those models. A custom depth function of a material becomes the depth bias
// of the record: the offset that it gives at the model's distance from the
// camera. It clears s.rigid. It does not allocate once its buffers have
// grown.
func (s *drawScratch) groupRigid(camera *Camera) {
	camPos := camera.WorldPosition()
	for i := range s.rigid {
		r := &s.rigid[i]
		g := r.part.meshGroup - 1
		if r.model.GPUBend != nil {
			// A bent model has uniforms of its own, so a draw of its own.
			g = len(s.meshDraws)
			s.meshDraws = append(s.meshDraws, meshDraw{part: r.part, model: r.model})
		} else if g >= 0 && !sameRGB(s.meshDraws[g].model, r.model) {
			g = -1
			for j := range s.meshDraws {
				if d := &s.meshDraws[j]; d.part == r.part && sameRGB(d.model, r.model) {
					g = j
					break
				}
			}
		}
		if g < 0 {
			g = len(s.meshDraws)
			s.meshDraws = append(s.meshDraws, meshDraw{part: r.part, model: r.model})
			r.part.meshGroup = g + 1
		}
		if r.model.GPUBend != nil {
			r.part.meshGroup = 0
		}
		r.group = g
		s.meshDraws[g].n++
	}
	s.rigidRecords = slices.Grow(s.rigidRecords[:0], len(s.rigid))[:len(s.rigid)]
	off := 0
	for i := range s.meshDraws {
		d := &s.meshDraws[i]
		if d.model != nil {
			d.records = s.rigidRecords[off : off : off+d.n]
			off += d.n
			d.part.meshGroup = 0
		}
	}
	for _, r := range s.rigid {
		d := &s.meshDraws[r.group]
		var bias float32
		if f := r.part.Material; f != nil && f.CustomDepthFunction != nil {
			z := camPos.DistanceTo(r.model.WorldPosition())
			bias = z - f.CustomDepthFunction(r.model, camera, r.part, 0, z)
		}
		d.records = append(d.records, rigidRecord(r.model, bias))
	}
	clear(s.rigid)
	s.rigid = s.rigid[:0]
}

// sameRGB reports whether the models a and b have the same colour, but for
// the alpha, which each record carries.
func sameRGB(a, b *Model) bool {
	return a.Color.R == b.Color.R && a.Color.G == b.Color.G && a.Color.B == b.Color.B
}

// RenderMeshes draws the solid, unlit meshes of the batches with a hardware
// depth buffer, for each record of each batch, into the colour and the depth
// textures of the camera, before Render draws the other models. Every part of
// each mesh must have a GPU mesh, see Mesh.BuildGPUMesh; the others are left
// out. Call it after Clear and before Render, and once in each frame: the
// depth buffers of the GPU clear only at the first draw of a frame. It does
// not allocate once its buffers have grown.
func (camera *Camera) RenderMeshes(scene *Scene, batches []MeshBatch) {
	draw := camera.appendBatches(batches)
	list := draw.meshDraws[draw.queued:]
	camera.drawMeshes(scene, list, false)
	clear(list)
	draw.meshDraws = draw.meshDraws[:draw.queued]
}

// QueueMeshes queues the batches for the next Render, which draws them as
// RenderMeshes does, first, in one run with its own parts on the mesh path:
// one clear of depthIntermediate, the depth of every part, one copy into the
// depth texture, and the colour of every part. The batches and their records
// must stay unchanged until Render. Call it after Clear and before Render, and
// once in each frame. It does not allocate once its buffers have grown.
func (camera *Camera) QueueMeshes(batches []MeshBatch) {
	draw := camera.appendBatches(batches)
	draw.queued = len(draw.meshDraws)
}

// appendBatches appends one draw for each part of each batch that has a GPU
// mesh to the queued draws, and returns the scratch of the camera.
func (camera *Camera) appendBatches(batches []MeshBatch) *drawScratch {
	if camera.draw == nil {
		camera.draw = newDrawScratch()
	}
	draw := camera.draw
	draw.meshDraws = draw.meshDraws[:draw.queued]
	for _, b := range batches {
		for _, part := range b.Mesh.MeshParts {
			if part.gpuMeshReady() && len(b.Records) > 0 {
				draw.meshDraws = append(draw.meshDraws, meshDraw{part: part, records: b.Records, pose: b.Pose})
			}
		}
	}
	return draw
}

// MeshStats returns the draws with a GPU mesh, and the instance records that
// they drew, since the last Clear.
func (camera *Camera) MeshStats() (draws, instances int) {
	return camera.meshDraws, camera.meshInstances
}

// clearDepthShaderSource clears every pixel of an image with a full-screen
// quad at the far plane. With the depth test, the quad is the first depth
// draw of the image in the frame, so the depth buffer clears at its start,
// the test keeps every fragment, and the quad writes the clear value back.
var clearDepthShaderSource = []byte(`//kage:unit pixels

package main

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	p := imageDstProjection() * vec4(dstPos+imageDstOrigin(), 0, 1)
	return vec4(p.xy, p.w, p.w), srcPos, color, custom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return vec4(0)
}
`)

// directDepthShaderSource is the fragment function of the depth pass of the
// mesh path with Camera.GPUMeshDirectDepth. It writes the depth of the
// fragment in the encoding of the depth texture. The hardware depth test
// keeps the nearest fragment, so the shader reads no image and does not
// discard.
var directDepthShaderSource = []byte(`//kage:unit pixels

package main

// encodeDepth takes the blue byte from the same product as the green byte,
// so that the two always carry together.
func encodeDepth(depth float) vec4 {
	r := floor(depth * 255) / 255
	g := fract(depth * 255) * 255
	return vec4(r, floor(g) / 255, fract(g), 1)
}

func Fragment(dstPos vec4, srcPos vec2, color, custom vec4) vec4 {
	return encodeDepth(custom.y)
}
`)

// resolveDepthShaderSource is the depth resolve of Camera.GPUMeshOnePass. It
// reads the hardware depth of the colour texture as image 0, see
// ebiten.DrawTrianglesShaderOptions.SourceDepth, and writes the depth in the
// encoding of the depth texture with an alpha of 1, or transparent black at
// the far plane, where nothing drew. ResolveClip turns the hardware depth z
// into the z/w of newGPUClipPlanes, q = z*x + y, and holds the near plane
// and (far-near)/far, so the distance along the view is
// near / (1 - q*(far-near)/far). ResolveDepth turns that distance into the
// depth of setGPUUniforms.
var resolveDepthShaderSource = []byte(`//kage:unit pixels

package main

var ResolveClip vec4
var ResolveDepth vec2

func encodeDepth(depth float) vec4 {
	r := floor(depth * 255) / 255
	g := fract(depth * 255) * 255
	return vec4(r, floor(g) / 255, fract(g), 1)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	z := imageSrc0UnsafeAt(srcPos).r
	if z >= 1 {
		return vec4(0)
	}
	q := z*ResolveClip.x + ResolveClip.y
	d := ResolveClip.z / (1 - q*ResolveClip.w)
	return encodeDepth(clamp(d*ResolveDepth.x+ResolveDepth.y, 0, 1))
}
`)

// resolveDepth draws the hardware depth of the colour texture into the depth
// texture, see resolveDepthShaderSource, for the projection proj and the
// depth encoding of drawMeshes. OpenGL, the only graphics library that
// ebiten.IsDepthSourceSupported allows, maps a z/w from -1 to 1 to a depth
// from 0 to 1. It does not allocate.
func (camera *Camera) resolveDepth(proj Matrix4, margin, spread float32) {
	draw := camera.draw
	src := camera.resultColorTexture
	w, h := float32(src.Bounds().Dx()), float32(src.Bounds().Dy())
	v := &draw.resolveVertices
	v[1].DstX, v[1].SrcX = w, w
	v[2].DstY, v[2].SrcY = h, h
	v[3].DstX, v[3].SrcX, v[3].DstY, v[3].SrcY = w, w, h, h
	c, d := draw.resolveClip, draw.resolveDepth
	c[0], c[1], c[2], c[3] = 2, -1, camera.near, (camera.far-camera.near)/camera.far
	// The clip z of a point at the distance d along the view is
	// -d*proj[2][2] + proj[3][2], see newGPUClip.
	d[0], d[1] = -proj[2][2]/spread, (proj[3][2]+margin)/spread
	opt := &draw.resolveOptions
	opt.Images[0] = src
	camera.resultDepthTexture.DrawTrianglesShader32(v[:], clearIndices[:], camera.resolveDepthShader, opt)
}

// clearForMeshes clears img to transparent black, as Clear does, with a draw
// that also starts the depth test of the frame. A tile-based GPU then draws
// the clear and the mesh draws after it in one render pass, in place of a
// pass for the clear and a pass that clears the depth buffer. img must be an
// unmanaged image, not a sub-image, and its first depth draw of the frame
// must come after this call. It does not allocate.
func (camera *Camera) clearForMeshes(img *ebiten.Image) {
	draw := camera.draw
	w, h := float32(img.Bounds().Dx()), float32(img.Bounds().Dy())
	v := &draw.clearVertices
	v[1].DstX, v[2].DstY = w, h
	v[3].DstX, v[3].DstY = w, h
	draw.clearOptions.Blend = ebiten.BlendCopy
	draw.clearOptions.Depth = true
	img.DrawTrianglesShader32(v[:], clearIndices[:], camera.clearShaderDepth, &draw.clearOptions)
}

// clearIndices are the two triangles of the quad of clearForMeshes.
var clearIndices = [6]uint32{0, 1, 2, 1, 3, 2}

// drawMeshes draws the list on the mesh path. The depth pass draws each part
// into depthIntermediate with a hardware depth test, and discards a fragment
// that is behind the depth texture, as the depth pass of the sorted path
// does, so that the parts stay behind the models and the shader draws that
// came before them. The depth texture then takes depthIntermediate, and the
// colour pass draws each part into the colour texture with a hardware depth
// test of its own, where depthIntermediate holds a depth. So each pixel takes
// the nearest part in both passes, with the same depth encoding as the
// sorted path.
//
// With GPUMeshDirectDepth, the depth pass draws each part straight into the
// depth texture with the hardware depth test against the depth buffer of
// the depth texture, and the colour pass reads the depth texture in place
// of depthIntermediate.
//
// With onePass, see Camera.GPUMeshOnePass, there is no depth pass: the
// colour pass draws each part with the shaders that have no gate, then
// Camera.AfterMeshColour runs, also when the list is empty, and then
// resolveDepth gives the depth texture the depth of the colour texture,
// unless the list is empty.
func (camera *Camera) drawMeshes(scene *Scene, list []meshDraw, onePass bool) {
	if len(list) == 0 {
		if onePass && camera.AfterMeshColour != nil {
			camera.AfterMeshColour()
		}
		return
	}
	draw := camera.draw
	vp := camera.ViewMatrix().Mult(camera.Projection())
	direct := (camera.GPUMeshDirectDepth || onePass) && camera.perspective
	clip := newGPUClip(camera.Projection(), camera.near, camera.far)
	if direct {
		clip = newGPUClipPlanes(camera.Projection(), camera.near, camera.far)
	}
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
		if d.model != nil {
			tint = NewColor4(d.model.Color.R, d.model.Color.G, d.model.Color.B, 1)
		}
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
		if d.model != nil && d.model.GPUBend != nil {
			draw.setBendUniforms(d.model.GPUBend)
		}
		draw.meshCull[0], draw.meshCull[1], draw.meshCull[2], draw.meshCull[3] = camPos.X, camPos.Y, camPos.Z, facing
	}

	depthDst, depthSrc := camera.depthIntermediate, camera.resultDepthTexture
	meshShader, rigidShader, bendShader, poseShader := camera.depthShaderMesh, camera.depthShaderRigid, camera.depthShaderBend, camera.depthShaderPose
	if direct {
		depthDst, depthSrc = camera.resultDepthTexture, nil
		meshShader, rigidShader, bendShader, poseShader = camera.directDepthMesh, camera.directDepthRigid, camera.directDepthBend, camera.directDepthPose
	} else {
		camera.clearForMeshes(camera.depthIntermediate)
	}
	// With onePass, the colour pass alone draws the parts.
	depthList := list
	if onePass {
		depthList = nil
	}
	opt := &draw.meshDepthOptions
	opt.Images[0] = depthSrc
	for i := range depthList {
		d := &depthList[i]
		uniforms(d)
		opt.Mesh = d.part.gpuMesh
		shader := meshShader
		if d.model != nil && d.model.GPUBend != nil {
			shader = bendShader
		} else if d.model != nil {
			shader = rigidShader
		} else if d.pose {
			shader = poseShader
		}
		depthDst.DrawTrianglesShader32(d.records, nil, shader, opt)
	}
	if !direct {
		camera.resultDepthTexture.DrawImage(camera.depthIntermediate, nil)
	}

	// The colour pass tests the depth with its own hardware depth buffer, so
	// it takes no depth gate from the last draw of the sorted path.
	draw.depthGate[0] = 0
	gate := depthDst
	colourMesh, colourRigid, colourBend, colourPose := camera.colorShaderMesh, camera.colorShaderRigid, camera.colorShaderBend, camera.colorShaderPose
	passes := 2
	if onePass {
		gate = nil
		colourMesh, colourRigid, colourBend, colourPose = camera.onePassMesh, camera.onePassRigid, camera.onePassBend, camera.onePassPose
		passes = 1
	}
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
		opt.Images[0], opt.Images[1] = img, gate
		opt.Mesh = d.part.gpuMesh
		shader := colourMesh
		if d.pose {
			shader = colourPose
		}
		if d.model != nil {
			variants := camera.rigidVariants(d.model)
			shader = colourRigid
			if d.model.GPUBend != nil {
				shader = colourBend
			}
			if mat.shaderOn() {
				// A fragment shader of a material, with its uniforms and
				// images, as on the sorted path.
				shader = variants[mat.fragmentShader]
				if o := mat.FragmentShaderOptions; o != nil {
					opt.Uniforms = draw.withFragmentUniforms(opt.Uniforms, o.Uniforms)
					for i, img := range o.Images {
						if img != nil {
							opt.Images[i] = img
						}
					}
				}
			}
		}
		camera.resultColorTexture.DrawTrianglesShader32(d.records, nil, shader, opt)

		camera.meshDraws += passes
		camera.meshInstances += len(d.records)
		if camera.DebugInfo.On {
			camera.DebugInfo.drawnParts++
			camera.DebugInfo.drawnTris += d.part.TriangleCount() * len(d.records)
		}
	}

	if onePass {
		if camera.AfterMeshColour != nil {
			camera.AfterMeshColour()
		}
		camera.resolveDepth(camera.Projection(), margin, spread)
	}
}
