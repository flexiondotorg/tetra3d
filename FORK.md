# Tetra3D fork for Five Horizons

This branch is a fork of [Tetra3D](https://github.com/SolarLune/tetra3d) for the game Five Horizons. The readme of the upstream project is `readme.md`.

- Upstream base: commit `18407def647e46ecd6d727079798ddffa68874ee` on `main`, the merge of pull request #46. It is the former base `6c1461c8f559d336e5fdadfb658f64f2a9dce453` plus our 14 commits (T01 to T14) and 8 merge commits, and nothing else. The set of T01 to T14 is now upstream, so this branch no longer carries it.
- Branch: `fivehorizons`, one commit for each patch, 33 patches on the upstream base, and one `docs` commit on top that updates this note. A rebase rewrites the branch. Each freeze is a tag.
- Planned tag: `fivehorizons-3`, for the first set on the base `18407de`. The tags `fivehorizons-1` and `fivehorizons-2` stay on the former base `6c1461c`.
- Licence: see `LICENSE`, unchanged from upstream.

## Upstream status

Upstream merged all eight of our pull requests. No pull request of ours is open. Each has a thank-you comment and no review. The issues #37, #39, #19, and #34 are closed. #34 is not ours.

| Patches | Pull request |
| --- | --- |
| T01 to T03, the per-frame costs of `ProcessVertices` | #38, for issue #37 |
| T04, the stable counting sort | #40, for issue #39 |
| T05, the original normals in `AddVertices` (A1) | #41 |
| T06, the depth test with three bytes (A2) | #42 |
| T07, the bounds of the texture wrap (A3) | #43 |
| T08, the atomic ID counters (A4) | #44 |
| T09, an error for invalid glTF vertex data (A5) | #45, which says `Fixes #19` |
| T10 to T14, the allocations of the render loop (B) | #46 |

T06 and T08 upstream differ from the old fork commits only by our own amendments. The patches T01 to T14 are dropped from this branch because the base has them.

## Patch order

1. T0 (T01 to T04) and T1 (T05 to T14): merged upstream, see "Upstream status". They are not on this branch.
2. T2 (T15 to T26): optimisations and small additions that change an API or a behaviour.
3. T3 (T27): a fork-only patch, the opt-in vertex counter behind the `vertexcount` build tag.
4. T4 (T28 to T36, and T47): the build patch, the mesh path (GPU-resident meshes, instanced draws, and a hardware depth buffer), and the depth test inside the parts of the sorted path.
5. After the tiers (T37 to T45, and T49): the deprecated Ebitengine API (T37 to T42), the fixes for tile-based GPUs (T43 to T45), and the unsorted triangle depth (T49).

Patch numbers have two digits, T01 to T49, and each number stays fixed. T46 and T48 are retired numbers with no commit of their own, see "Squashed patches". Tiers have one digit, T0 to T4.

T01 to T27 build against stock Ebitengine. T28 adds the `replace` line of `go.mod` for the Ebitengine fork [`flexiondotorg/ebiten`](https://github.com/flexiondotorg/ebiten), branch `fivehorizons`, and the patches after it build against the fork. T29 to T36, T43, T44, and T47 need it, and T27 is a diagnostic.

## Squashed patches

T46 and T48 are squashed into T47, commit `22e7e49`, because T47 builds on T46.

- T46 was the one pass of the mesh path, `Camera.GPUMeshOnePass`. The parts drew their colour once with no depth pass, and a full-screen draw resolved the hardware depth of the colour texture into the depth texture.
- T47 is the hardware depth of the colour texture, `Camera.HardwareDepth`. `Render` draws the mesh path in one pass, runs `Camera.AfterMeshColour`, and draws each part of the sorted path once into the colour texture with the hardware depth test. Solid parts test and write the depth, alpha-clip parts also discard, and transparent parts test without a write, back to front. There is no depth texture, no `depthIntermediate`, no clear, no copy, and no resolve. It needs `DrawTrianglesShaderOptions.DepthReadOnly` and `ImageDepth` of the Ebitengine fork, which also maps the clip z of a vertex function to a depth from 0 to 1 on OpenGL.
- T48 was the removal of `Camera.GPUMeshOnePass`, its depth resolve, and its shader. The colour shaders of the mesh path with no gate belong to `Camera.HardwareDepth`, which keeps `Camera.AfterMeshColour`.

`Camera.GPUMeshOnePass` never exists on this branch. Without `HardwareDepth`, the mesh path draws its depth pass as before.

## Patches

| Patch | Commit | Subject |
| --- | --- | --- |
| T15 | `8ccb0e8` | perf: transform each vertex once with clip codes |
| T16 | `5fe2678` | perf: draw mesh parts with indexed vertices |
| T17 | `bccdcf7` | perf: carry the depth in Custom1 of the colour vertices |
| T18 | `b4c1bb6` | perf: transform the common path's vertices in one plain loop |
| T19 | `16e8708` | perf: write the vertex lists in mesh order, each colour and UV read once |
| T20 | `ed84f41` | perf: write the common path's vertex lists in one plain loop |
| T21 | `bf58eda` | perf: read packed triangle indices and centres in the triangle pass |
| T22 | `d047df6` | perf: store a triangle index in each sorting triangle |
| T23 | `9d8ac68` | perf: size the per-vertex buffers at the vertex count, with bones only for meshes with bones |
| T24 | `0f36e7b` | perf: pack a triangle into 80 bytes |
| T25 | `ef5b45b` | feat: Camera.SetDepthTexture takes a depth texture of the caller |
| T26 | `61e5536` | feat: Camera.ResetFrame starts a frame without a clear |
| T27 | `f103465` | feat: an opt-in vertex counter hook behind the vertexcount tag |
| T28 | `d22e712` | build: build against the Ebitengine fork |
| T29 | `361765f` | feat: draw solid mesh batches with a hardware depth buffer |
| T30 | `0482670` | feat: draw models on the mesh path with one record each |
| T31 | `b48fc3c` | feat: move a mesh between two poses on the mesh path |
| T32 | `b6b3a2c` | feat: bend a model about two joints on the mesh path |
| T33 | `20c473b` | perf: queue the mesh batches for Render |
| T34 | `53b45af` | perf: clear the depth run of the mesh path in the same render pass |
| T35 | `3d1857f` | feat: test the depth inside each solid part of the sorted path |
| T36 | `45d98c2` | perf: draw the depth of a run of solid parts before their colour |
| T37 | `6c13ff7` | refactor: replace the deprecated Dispose and Size calls of Ebitengine |
| T38 | `3d8324d` | refactor: stop passing the deprecated AntiAlias and FillRule to the colour pass |
| T39 | `52eaac7` | refactor: rename the deprecated Kage builtin imageSrc1UnsafeAt |
| T40 | `442cea0` | refactor: draw text with text/v2 in place of the deprecated text package |
| T41 | `131eb92` | refactor: replace the deprecated Ebitengine API in the examples |
| T42 | `113ad85` | refactor: keep the deprecated Ebitengine option fields out of the public API |
| T43 | `9eba9b1` | perf: return a transparent pixel in place of a discard in the mesh colour pass |
| T44 | `6152e6c` | perf: add GPUMeshDirectDepth to draw the mesh depth pass into the depth texture |
| T45 | `c0f46c4` | perf: skip the fog work of the colour shader when the fog is off |
| T47 | `22e7e49` | perf: add HardwareDepth to draw the sorted path with the hardware depth test |
| T49 | `a13af42` | perf: skip the triangle depth of a part that draws unsorted |

T49 skips the depth of each triangle when the part draws its triangles unsorted: a part with the sort mode `TriangleSortModeNone`, which includes every solid part under the depth test, needs no distance from the camera, so `processVertices` no longer reads the triangle centres or computes the distance for it. On the CPU processing path of Five Horizons at 2150 m, the flat time of `processVertices` falls by about a quarter.

## Plan for the remaining upstream offers

Pace: one or two pull requests at a time. The maintainer discussed the method of these patches with Martin on Discord and merged all eight pull requests.

| Wave | Patches | Offer | Issue first | Before sending |
| --- | --- | --- | --- | --- |
| 1 | T45, the fog skip | One pull request | No | Measure T45 alone on `18407de` with the stock scene on a real display. |
| 1 | T37, T38, T40, T41, the deprecated API | One pull request | No | Re-cut the `ownDepthTexture` line of T37, because T25 adds that field. Take the upstream `go.mod` and `go.sum` and run `go mod tidy` for T40. Check screenshots. |
| 1 | T49, the unsorted triangle depth | One pull request | No | Try it on `18407de` alone. Measure it alone with the stock scene. |
| 2 | T15 to T20, the vertex pass and the indexed output | An issue, then a series of pull requests | Yes | Ask the maintainer about indexed rendering, which upstream tried before, and about the `uint16` index cap. |
| 2 | T21 to T24, the packed triangles | A pull request after T15 to T20 | Yes | It adds `Mesh.UpdateTriangleData()`. `VertexBones` and `VertexWeights` are empty for a mesh without bones. |
| 2 | T25 and T26, `SetDepthTexture` and `ResetFrame` | One pull request | Yes | The API is public. It is independent now that B is upstream. |
| Hold | T39, the Kage rename | After upstream moves to Ebitengine v2.10 | A proposal to bump | Upstream is on v2.9.7. |
| Hold | T42, the Tetra3D option types | A proposal issue | Yes | It breaks the API. |
| Fork only | T27, T28, T29 to T36, T43, T44, T47 | None | Not applicable | They need the Ebitengine fork or are diagnostics. |

The numbers that the fork recorded for T45 (about -4.4% frame time, with T43 and T44, in `docs/devlog/079` of the game) and T49 (`docs/devlog/081`) come from the fork with its fork-only patches. They are not evidence for upstream.
