package tetra3d

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// screenBounds holds the smallest and largest screen positions of a set of
// vertices, leaving out the positions that are not a number. nan is true when
// a position is not a number.
type screenBounds struct {
	minX, minY, maxX, maxY float32
	nan                    bool
}

// emptyScreenBounds returns the bounds of no vertices.
func emptyScreenBounds() screenBounds {
	inf := float32(math.Inf(1))
	return screenBounds{minX: inf, minY: inf, maxX: -inf, maxY: -inf}
}

// add extends b to include o.
func (b *screenBounds) add(o screenBounds) {
	b.minX = min(b.minX, o.minX)
	b.minY = min(b.minY, o.minY)
	b.maxX = max(b.maxX, o.maxX)
	b.maxY = max(b.maxY, o.maxY)
	b.nan = b.nan || o.nan
}

// rect returns the pixels inside b, plus a margin of one pixel, clipped to a
// w by h target. partial is false when the rectangle is the full target, or
// when a position is not a number.
func (b screenBounds) rect(w, h int) (rect image.Rectangle, partial bool) {
	if b.nan {
		return image.Rectangle{}, false
	}
	rect = image.Rect(clampPixel(b.minX-1, w), clampPixel(b.minY-1, h), clampPixel(b.maxX+2, w), clampPixel(b.maxY+2, h))
	return rect, rect != image.Rect(0, 0, w, h)
}

// writeVertexList writes the colour vertex of each vertex from start to end
// whose entry in stamps equals stamp, in mesh order, from out[n] on, and puts
// the place of each one in slots. It returns the new length of the list and
// the bounds of the screen positions that it wrote.
//
// It is the list write of Camera.Render for a perspective camera, with the
// screen positions of the vertex pass, depth render on, billboarded depth,
// and no vertex clip function, lighting, normals render, perspective-corrected
// texture mapping, or custom depth function. colors is nil for a mesh with no
// vertex colours. The arithmetic is the same as the general loop in
// Camera.Render, so the results are the same to the bit.
func writeVertexList(out []ebiten.Vertex, n, start, end int, stamp uint32, stamps []uint32, slots []int32,
	clip []Vector4, screen []Vector2, uvs []Vector2, colors []Color4, tint Color4,
	camWidth, camHeight int, srcW, srcH, depthMargin, camSpread float32) (int, screenBounds) {

	halfCamWidth, halfCamHeight := float32(camWidth)/2, float32(camHeight)/2
	camW, camH := float32(camWidth), float32(camHeight)

	stamps = stamps[start:end]
	slots = slots[start:end]
	clip = clip[start:end]
	screen = screen[start:end]
	uvs = uvs[start:end]
	hasColors := colors != nil
	if hasColors {
		colors = colors[start:end]
	}

	b := emptyScreenBounds()

	for i := range stamps {

		if stamps[i] != stamp {
			continue
		}
		slots[i] = int32(n)
		o := &out[n]

		t := clip[i]
		w := t.W

		var dx, dy float32

		if w >= 0 {
			s := screen[i]
			dx = float32(s.X*camW + halfCamWidth)
			dy = float32((-s.Y)*camH + halfCamHeight)
		} else {
			w = 0.001
			dx = float32((t.X/w)*camW + halfCamWidth)
			dy = float32((t.Y/-w)*camH + halfCamHeight)
		}

		o.DstX = dx
		o.DstY = dy

		if dx != dx || dy != dy {
			b.nan = true
		}
		if dx < b.minX {
			b.minX = dx
		}
		if dx > b.maxX {
			b.maxX = dx
		}
		if dy < b.minY {
			b.minY = dy
		}
		if dy > b.maxY {
			b.maxY = dy
		}

		uv := uvs[i]
		o.SrcX = float32(uv.X * srcW)
		o.SrcY = float32((1 - uv.Y) * srcH)

		if hasColors {
			c := colors[i]
			o.ColorR = c.R * tint.R
			o.ColorG = c.G * tint.G
			o.ColorB = c.B * tint.B
			o.ColorA = c.A * tint.A
		} else {
			o.ColorR = tint.R
			o.ColorG = tint.G
			o.ColorB = tint.B
			o.ColorA = tint.A
		}

		depth := (t.Z + depthMargin) / camSpread
		if depth < 0 {
			depth = 0
		} else if depth > 1 {
			depth = 1
		}
		o.Custom1 = float32(depth)

		n++
	}

	return n, b
}
