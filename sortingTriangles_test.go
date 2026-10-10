package tetra3d

import (
	"math/rand"
	"testing"

	"github.com/solarlune/tetra3d/math32"
)

// oldBucketSort gives the draw order of the bucket sort with one buffer for each bin.
func oldBucketSort(tris []*Triangle, depths []float32, sortMode, binCount int, minRange, maxRange float32) []*Triangle {

	bins := make([][]*Triangle, binCount)

	rangeDiff := maxRange - minRange
	if rangeDiff == 0 {
		rangeDiff += 0.001
	}

	for i, tri := range tris {
		targetBin := 0
		if sortMode != TriangleSortModeNone && binCount > 1 {
			depth := (depths[i] - minRange) / rangeDiff * float32(binCount)
			targetBin = int(math32.Clamp(depth, 0, float32(binCount-1)))
		}
		bins[targetBin] = append(bins[targetBin], tri)
	}

	out := []*Triangle{}
	if sortMode == TriangleSortModeBackToFront {
		for b := binCount - 1; b >= 0; b-- {
			out = append(out, bins[b]...)
		}
	} else {
		for b := 0; b < binCount; b++ {
			out = append(out, bins[b]...)
		}
	}
	return out

}

func TestSortingTriangleBucketOrder(t *testing.T) {

	random := rand.New(rand.NewSource(1))

	for _, binCount := range []int{1, 8, 512} {

		for _, sortMode := range []int{TriangleSortModeBackToFront, TriangleSortModeFrontToBack, TriangleSortModeNone} {

			bucket := newSortingTriangleBucket()
			bucket.Resize(binCount)
			bucket.sortMode = sortMode

			// Sort more than once to check that the buffers are reused correctly.
			for _, triCount := range []int{2000, 40000, 700, insertionSortMaxTris, 12, insertionSortMaxTris + 1, 1} {

				tris := make([]*Triangle, triCount)
				depths := make([]float32, triCount)
				// The bucket indexes tris, as setMesh does for a mesh.
				bucket.tris = tris

				for i := range tris {
					tris[i] = &Triangle{id: uint32(i)}
					// Few distinct depths give many ties; some fall outside the range.
					depths[i] = float32(random.Intn(40)) - 5
					bucket.AddTriangle(i, depths[i])
				}

				bucket.Sort(0, 30)

				want := oldBucketSort(tris, depths, sortMode, binCount, 0, 30)

				got := []*Triangle{}
				bucket.ForEach(func(triIndex int, triangle *Triangle) {
					if triIndex != len(got) {
						t.Fatalf("bins %d, mode %d: triIndex %d, want %d", binCount, sortMode, triIndex, len(got))
					}
					got = append(got, triangle)
				})

				if len(got) != len(want) {
					t.Fatalf("bins %d, mode %d: %d triangles, want %d", binCount, sortMode, len(got), len(want))
				}

				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("bins %d, mode %d: triangle %d is %d, want %d", binCount, sortMode, i, got[i].id, want[i].id)
					}
				}

				bucket.Clear()

			}

		}

	}

}

func TestSortingTriangleBucketInsertionSort(t *testing.T) {

	random := rand.New(rand.NewSource(1))

	for _, binCount := range []int{2, 8, 512} {

		for _, sortMode := range []int{TriangleSortModeBackToFront, TriangleSortModeFrontToBack} {

			bucket := newSortingTriangleBucket()
			bucket.Resize(binCount)
			bucket.sortMode = sortMode
			bucket.binOf = make([]int32, 4*insertionSortMaxTris)
			bucket.sortedTris = make([]sortingTriangle, 4*insertionSortMaxTris)

			for triCount := 0; triCount <= 4*insertionSortMaxTris; triCount++ {

				// Few distinct depths give many equal keys; some fall outside the range.
				for _, distinct := range []int{1, 3, 40, 1 << 20} {

					for i := range triCount {
						depth := float32(random.Intn(distinct))/float32(distinct)*40 - 5
						bucket.AddTriangle(i, depth)
					}

					bucket.countingSort(triCount, 0, 30)
					want := append([]sortingTriangle{}, bucket.sorted...)

					bucket.insertionSort(triCount, 0, 30)

					if len(bucket.sorted) != len(want) {
						t.Fatalf("bins %d, mode %d, %d triangles: %d sorted", binCount, sortMode, triCount, len(bucket.sorted))
					}

					for i := range want {
						if bucket.sorted[i] != want[i] {
							t.Fatalf("bins %d, mode %d, %d triangles: triangle %d is %d, want %d", binCount, sortMode, triCount, i, bucket.sorted[i].index, want[i].index)
						}
					}

					bucket.Clear()

				}

			}

		}

	}

}
