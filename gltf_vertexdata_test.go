package tetra3d

import (
	"bytes"
	"math"
	"testing"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// vertexDataGLTF builds a glTF with one triangle of three vertices.
// The attributes function adds extra attributes and can change the indices.
func vertexDataGLTF(t *testing.T, indices []uint16, attributes func(doc *gltf.Document, attrs gltf.PrimitiveAttributes)) []byte {
	t.Helper()
	doc := gltf.NewDocument()
	attrs := gltf.PrimitiveAttributes{
		gltf.POSITION: modeler.WritePosition(doc, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}),
	}
	if attributes != nil {
		attributes(doc, attrs)
	}
	idx := modeler.WriteIndices(doc, indices)
	doc.Meshes = append(doc.Meshes, &gltf.Mesh{Name: "m", Primitives: []*gltf.Primitive{{
		Attributes: attrs, Indices: gltf.Index(idx),
	}}})
	doc.Nodes = append(doc.Nodes, &gltf.Node{Name: "n", Mesh: gltf.Index(0)})
	doc.Scenes = append(doc.Scenes, &gltf.Scene{Nodes: []int{0}})
	doc.Scene = gltf.Index(0)
	var buf bytes.Buffer
	if err := gltf.NewEncoder(&buf).Encode(doc); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLoadGLTFDataInvalidVertexData(t *testing.T) {
	triangle := []uint16{0, 1, 2}
	cases := []struct {
		name       string
		indices    []uint16
		attributes func(doc *gltf.Document, attrs gltf.PrimitiveAttributes)
	}{
		{"index out of bounds", []uint16{0, 1, 5}, nil},
		{"TEXCOORD_0 longer than POSITION", triangle, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
			attrs[gltf.TEXCOORD_0] = modeler.WriteTextureCoord(doc, [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}})
		}},
		{"NORMAL longer than POSITION", triangle, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
			attrs[gltf.NORMAL] = modeler.WriteNormal(doc, [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}})
		}},
		{"COLOR_0 longer than POSITION", triangle, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
			attrs["COLOR_0"] = modeler.WriteColor(doc, [][4]uint8{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}})
		}},
		{"WEIGHTS_0 longer than POSITION", triangle, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
			attrs[gltf.WEIGHTS_0] = modeler.WriteWeights(doc, [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}})
			attrs[gltf.JOINTS_0] = modeler.WriteJoints(doc, [][4]uint16{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}})
		}},
		{"JOINTS_0 shorter than WEIGHTS_0", triangle, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
			attrs[gltf.WEIGHTS_0] = modeler.WriteWeights(doc, [][4]float32{{1, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 0, 0}})
			attrs[gltf.JOINTS_0] = modeler.WriteJoints(doc, [][4]uint16{{0, 0, 0, 0}, {0, 0, 0, 0}})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := vertexDataGLTF(t, c.indices, c.attributes)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("LoadGLTFData panicked: %v", r)
				}
			}()
			if _, err := LoadGLTFData(bytes.NewReader(data), nil); err == nil {
				t.Fatal("LoadGLTFData returned no error")
			}
		})
	}
}

func TestLoadGLTFDataValidVertexData(t *testing.T) {
	data := vertexDataGLTF(t, []uint16{0, 1, 2}, func(doc *gltf.Document, attrs gltf.PrimitiveAttributes) {
		attrs[gltf.TEXCOORD_0] = modeler.WriteTextureCoord(doc, [][2]float32{{0, 0}, {1, 0}, {0, 1}})
		attrs[gltf.NORMAL] = modeler.WriteNormal(doc, [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}})
		attrs["COLOR_0"] = modeler.WriteColor(doc, [][4]uint8{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}})
	})
	if _, err := LoadGLTFData(bytes.NewReader(data), nil); err != nil {
		t.Fatal(err)
	}
}

func TestSRGBFromByteMatchesConvertTosRGB(t *testing.T) {
	for i := range 256 {
		v := float32(i) / math.MaxUint8
		want := NewColor4(v, v, v, 1).ConvertTosRGB()
		got := sRGBFromByte[i]
		for _, w := range []float32{want.R, want.G, want.B} {
			if math.Float32bits(got) != math.Float32bits(w) {
				t.Fatalf("entry %d: got %v, want %v", i, got, w)
			}
		}
	}
}

func TestLoadGLTFDataVertexColoursAllBytes(t *testing.T) {
	const count = 258
	doc := gltf.NewDocument()
	positions := make([][3]float32, count)
	colours := make([][4]uint8, count)
	indices := make([]uint16, count)
	for i := range count {
		positions[i] = [3]float32{float32(i), float32(i % 3), 0}
		k := uint8(i)
		colours[i] = [4]uint8{k, 255 - k, k * 7, k ^ 0x5a}
		indices[i] = uint16(i)
	}
	attrs := gltf.PrimitiveAttributes{
		gltf.POSITION: modeler.WritePosition(doc, positions),
		"COLOR_0":     modeler.WriteColor(doc, colours),
	}
	idx := modeler.WriteIndices(doc, indices)
	doc.Meshes = append(doc.Meshes, &gltf.Mesh{Name: "m", Primitives: []*gltf.Primitive{{
		Attributes: attrs, Indices: gltf.Index(idx),
	}}})
	doc.Nodes = append(doc.Nodes, &gltf.Node{Name: "n", Mesh: gltf.Index(0)})
	doc.Scenes = append(doc.Scenes, &gltf.Scene{Nodes: []int{0}})
	doc.Scene = gltf.Index(0)
	var buf bytes.Buffer
	if err := gltf.NewEncoder(&buf).Encode(doc); err != nil {
		t.Fatal(err)
	}
	lib, err := LoadGLTFData(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := lib.MeshByName("m").VertexColors[0].Colors()
	if len(got) != count {
		t.Fatalf("got %d vertex colours, want %d", len(got), count)
	}
	for i, c := range colours {
		want := NewColor4(
			float32(c[0])/math.MaxUint8,
			float32(c[1])/math.MaxUint8,
			float32(c[2])/math.MaxUint8,
			float32(c[3])/math.MaxUint8,
		).ConvertTosRGB()
		g := got[i]
		for ch, pair := range [4][2]float32{{g.R, want.R}, {g.G, want.G}, {g.B, want.B}, {g.A, want.A}} {
			if math.Float32bits(pair[0]) != math.Float32bits(pair[1]) {
				t.Fatalf("vertex %d channel %d: got %v, want %v", i, ch, pair[0], pair[1])
			}
		}
	}
}
