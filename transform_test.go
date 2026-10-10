package tetra3d

import (
	"math"
	"math/rand/v2"
	"testing"
)

// oldTRS is the local transform of Node.Transform before newMatrix4TRS.
func oldTRS(position Vector3, rotation Matrix4, scale Vector3) Matrix4 {
	transform := NewMatrix4Scale(scale.X, scale.Y, scale.Z)
	transform = transform.Mult(rotation)
	return transform.Mult(NewMatrix4Translate(position.X, position.Y, position.Z))
}

// oldToMatrix4 is Quaternion.ToMatrix4 before the closed form.
func oldToMatrix4(quat Quaternion) Matrix4 {
	m1 := Matrix4{
		{quat.W, quat.Z, -quat.Y, quat.X},
		{-quat.Z, quat.W, quat.X, quat.Y},
		{quat.Y, -quat.X, quat.W, quat.Z},
		{-quat.X, -quat.Y, -quat.Z, quat.W},
	}
	m2 := Matrix4{
		{quat.W, quat.Z, -quat.Y, -quat.X},
		{-quat.Z, quat.W, quat.X, -quat.Y},
		{quat.Y, -quat.X, quat.W, -quat.Z},
		{quat.X, quat.Y, quat.Z, quat.W},
	}
	return m1.Mult(m2)
}

func sameBits(a, b Matrix4) bool {
	for i := range 4 {
		for j := range 4 {
			if math.Float32bits(a[i][j]) != math.Float32bits(b[i][j]) {
				return false
			}
		}
	}
	return true
}

var negZero = float32(math.Copysign(0, -1))

// edgeValues are finite values that exercise signed zeros, units, and a wide range of magnitudes.
var edgeValues = []float32{0, negZero, 1, -1, 0.5, -0.5, 2, -3, 1e-20, -1e-20, 1e-40, -1e-40, 1e15, -1e15, 0.1, -0.7}

func randomValue(rng *rand.Rand) float32 {
	if rng.IntN(3) == 0 {
		return edgeValues[rng.IntN(len(edgeValues))]
	}
	return float32((rng.Float64()*2 - 1) * math.Pow(10, float64(rng.IntN(9)-4)))
}

func randomQuaternion(rng *rand.Rand) Quaternion {
	q := Quaternion{randomValue(rng), randomValue(rng), randomValue(rng), randomValue(rng)}
	if rng.IntN(2) == 0 {
		q = q.Unit()
	}
	return q
}

func randomRotation(rng *rand.Rand) Matrix4 {
	switch rng.IntN(5) {
	case 0:
		return NewMatrix4()
	case 1:
		return oldToMatrix4(randomQuaternion(rng))
	case 2:
		return NewMatrix4Rotate(randomValue(rng), randomValue(rng), randomValue(rng), randomValue(rng))
	case 3:
		// An axis permutation with signed zeros and signed units.
		var m Matrix4
		perm := rng.Perm(3)
		for i := range 3 {
			for j := range 3 {
				m[i][j] = []float32{0, negZero}[rng.IntN(2)]
			}
			m[i][perm[i]] = []float32{1, -1}[rng.IntN(2)]
		}
		m[3] = [4]float32{[]float32{0, negZero}[rng.IntN(2)], []float32{0, negZero}[rng.IntN(2)], []float32{0, negZero}[rng.IntN(2)], 1}
		return m
	default:
		var m Matrix4
		for i := range 4 {
			for j := range 4 {
				m[i][j] = randomValue(rng)
			}
		}
		return m
	}
}

func randomVector(rng *rand.Rand) Vector3 {
	return Vector3{X: randomValue(rng), Y: randomValue(rng), Z: randomValue(rng)}
}

func finite(m Matrix4) bool {
	for i := range 4 {
		for j := range 4 {
			if math.IsInf(float64(m[i][j]), 0) || math.IsNaN(float64(m[i][j])) {
				return false
			}
		}
	}
	return true
}

func TestNewMatrix4TRSBits(t *testing.T) {
	rng := rand.New(rand.NewPCG(16, 1))
	checked := 0
	for range 2000000 {
		position, rotation, scale := randomVector(rng), randomRotation(rng), randomVector(rng)
		want := oldTRS(position, rotation, scale)
		if !finite(rotation) || !finite(want) {
			continue
		}
		checked++
		if got := newMatrix4TRS(position, rotation, scale); !sameBits(got, want) {
			t.Fatalf("position %v rotation %v scale %v: got %v, want %v", position, rotation, scale, got, want)
		}
	}
	t.Logf("%d finite cases have the same bits", checked)
}

func TestToMatrix4Bits(t *testing.T) {
	rng := rand.New(rand.NewPCG(16, 2))
	for range 2000000 {
		q := randomQuaternion(rng)
		want := oldToMatrix4(q)
		if !finite(want) {
			continue
		}
		if got := q.ToMatrix4(); !sameBits(got, want) {
			t.Fatalf("quaternion %v: got %v, want %v", q, got, want)
		}
	}
}
