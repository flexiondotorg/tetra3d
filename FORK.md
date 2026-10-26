# Tetra3D fork for Five Horizons

This branch is a fork of [Tetra3D](https://github.com/SolarLune/tetra3d) for the game Five Horizons. The readme of the upstream project is `readme.md`.

- Upstream base: commit `005dd7111b83b3455b228d0632721ea1bcdf2909` on `main`, the merge of pull request #49. It is the former base `18407def647e46ecd6d727079798ddffa68874ee` plus the upstream commits of T37, T38, T40, T41, T45, and T50, their 3 merge commits, and one commit of the maintainer, `487e3d5` "More changes!". The set of T37, T38, T40, T41, T45, and T50 is now upstream, so this branch no longer carries it.
- Branch: `fivehorizons`, one commit for each patch, 36 patches on the upstream base, two `build` commits for the Ebitengine pins, and the `docs` commits that update this note. A rebase rewrites the branch. Each freeze is a tag.
- Tags: `fivehorizons-4` is the first set on the base `005dd71`. `fivehorizons-5` adds build commit `901da3a`, which pins the Ebitengine fork freeze `fivehorizons-5`. `fivehorizons-3` is the first set on the former base `18407de`. The tags `fivehorizons-1` and `fivehorizons-2` stay on the base before it, `6c1461c`. The tag `pre-rebase-005dd71` keeps the former tip `6d78389`, which the game pins until the next freeze.
- Licence: see `LICENSE`, unchanged from upstream.

## Upstream status

Upstream merged eleven of our pull requests. The first eight each have a thank-you comment and no review. The issues #37, #39, #19, and #34 are closed. #34 is not ours. One pull request of ours is open, #51, and its issue #50 is open.

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
| T37, T38, T40, T41, the deprecated API | #47, merged on 2026-10-14 |
| T45, the fog skip | #48, merged on 2026-10-14 |
| T50, an error for an invalid glTF texture index | #49, which says `Fixes #18`, merged on 2026-10-14 |

T06 and T08 upstream differ from the old fork commits only by our own amendments. The patches T01 to T14, T37, T38, T40, T41, T45, and T50 are dropped from this branch because the base has them.

The upstream form of T37 uses `resultDepthTexture` where this branch uses `ownDepthTexture` of T25. The upstream form of T40 has the module versions of upstream, and the upstream form of T45 adds `frame_test.go`, which T25 also adds. This branch keeps its own form of each.

T45 showed no measurable change in the stock `lighting` scene on an AMD Radeon 8060S. The median ratio of patch to base frame time was 0.97 at 1920x1080 over 5 pairs and 0.9989 at 3840x2160 over 7 pairs.

### Open pull requests

| Patches | Pull request | State |
| --- | --- | --- |
| T15 and T18, the vertex pass with clip codes and a plain loop | #51, branch `vertex-pass-clip-codes` at `00b674c` on base `005dd71` | Open and ready for review since 2026-10-15, with no comment and no review |

Issue #50, "Proposal: a vertex pass and indexed output for faster CPU rendering", opened on 2026-10-15, covers T15 to T24 and T49 after T21 and T22. It is open, with no comment.

Pull request #51 opened as a draft and became ready for review on 2026-10-15. On the stock example scenes the gain was not significant over 9 rounds, because the examples have few vertices. The pull request states the game figure of about 2 ms a frame, which is about 120 ms each second at 60 fps.

## Patch order

1. T0 (T01 to T04) and T1 (T05 to T14): merged upstream, see "Upstream status". They are not on this branch.
2. T2 (T15 to T26): optimisations and small additions that change an API or a behaviour.
3. T3 (T27): a fork-only patch, the opt-in vertex counter behind the `vertexcount` build tag.
4. T4 (T28 to T36, and T47): the build patch, the mesh path (GPU-resident meshes, instanced draws, and a hardware depth buffer), and the depth test inside the parts of the sorted path.
5. After the tiers (T39, T42 to T44, and T49): the deprecated Ebitengine API (T39 and T42), the fixes for tile-based GPUs (T43 and T44), and the unsorted triangle depth (T49). T37, T38, T40, T41, T45, and T50 are merged upstream and are not on this branch.
6. The performance audit (T51 to T58), after the build commit that pins the Ebitengine fork freeze `fivehorizons-4`.
7. The build commit `901da3a`, which pins the Ebitengine fork freeze `fivehorizons-5`.

Patch numbers have two digits, T01 to T58, and each number stays fixed. T46 and T48 are retired numbers with no commit of their own, see "Squashed patches". Tiers have one digit, T0 to T4.

T01 to T27 build against stock Ebitengine. T28 adds the `replace` line of `go.mod` for the Ebitengine fork [`flexiondotorg/ebiten`](https://github.com/flexiondotorg/ebiten), branch `fivehorizons`, and the patches after it build against the fork. T29 to T36, T43, T44, and T47 need it, and T27 is a diagnostic.

## Squashed patches

T46 and T48 are squashed into T47, commit `f9ce200`, because T47 builds on T46.

- T46 was the one pass of the mesh path, `Camera.GPUMeshOnePass`. The parts drew their colour once with no depth pass, and a full-screen draw resolved the hardware depth of the colour texture into the depth texture.
- T47 is the hardware depth of the colour texture, `Camera.HardwareDepth`. `Render` draws the mesh path in one pass, runs `Camera.AfterMeshColour`, and draws each part of the sorted path once into the colour texture with the hardware depth test. Solid parts test and write the depth, alpha-clip parts also discard, and transparent parts test without a write, back to front. There is no depth texture, no `depthIntermediate`, no clear, no copy, and no resolve. It needs `DrawTrianglesShaderOptions.DepthReadOnly` and `ImageDepth` of the Ebitengine fork, which also maps the clip z of a vertex function to a depth from 0 to 1 on OpenGL.
- T48 was the removal of `Camera.GPUMeshOnePass`, its depth resolve, and its shader. The colour shaders of the mesh path with no gate belong to `Camera.HardwareDepth`, which keeps `Camera.AfterMeshColour`.

`Camera.GPUMeshOnePass` never exists on this branch. Without `HardwareDepth`, the mesh path draws its depth pass as before.

## Patches

| Patch | Commit | Subject |
| --- | --- | --- |
| T15 | `586af10` | perf: transform each vertex once with clip codes |
| T16 | `70f3199` | perf: draw mesh parts with indexed vertices |
| T17 | `1dd7914` | perf: carry the depth in Custom1 of the colour vertices |
| T18 | `faafbac` | perf: transform the common path's vertices in one plain loop |
| T19 | `18215fd` | perf: write the vertex lists in mesh order, each colour and UV read once |
| T20 | `a575706` | perf: write the common path's vertex lists in one plain loop |
| T21 | `1439582` | perf: read packed triangle indices and centres in the triangle pass |
| T22 | `0bcf429` | perf: store a triangle index in each sorting triangle |
| T23 | `a255743` | perf: size the per-vertex buffers at the vertex count, with bones only for meshes with bones |
| T24 | `6e6c8de` | perf: pack a triangle into 80 bytes |
| T25 | `2366f22` | feat: Camera.SetDepthTexture takes a depth texture of the caller |
| T26 | `1194d32` | feat: Camera.ResetFrame starts a frame without a clear |
| T27 | `6b7713b` | feat: an opt-in vertex counter hook behind the vertexcount tag |
| T28 | `37e16d9` | build: build against the Ebitengine fork |
| T29 | `ceb5ae2` | feat: draw solid mesh batches with a hardware depth buffer |
| T30 | `791e407` | feat: draw models on the mesh path with one record each |
| T31 | `6f664ef` | feat: move a mesh between two poses on the mesh path |
| T32 | `493deb6` | feat: bend a model about two joints on the mesh path |
| T33 | `cb7de21` | perf: queue the mesh batches for Render |
| T34 | `76e263b` | perf: clear the depth run of the mesh path in the same render pass |
| T35 | `d6a9dbf` | feat: test the depth inside each solid part of the sorted path |
| T36 | `147ad19` | perf: draw the depth of a run of solid parts before their colour |
| T39 | `214ed32` | refactor: rename the deprecated Kage builtin imageSrc1UnsafeAt |
| T42 | `6ecd558` | refactor: keep the deprecated Ebitengine option fields out of the public API |
| T43 | `23f359b` | perf: return a transparent pixel in place of a discard in the mesh colour pass |
| T44 | `e047498` | perf: add GPUMeshDirectDepth to draw the mesh depth pass into the depth texture |
| T47 | `f9ce200` | perf: add HardwareDepth to draw the sorted path with the hardware depth test |
| T49 | `a7b9e67` | perf: skip the triangle depth of a part that draws unsorted |
| T51 | `f4dfbb0` | perf: convert vertex colours to sRGB with a lookup table |
| T52 | `71934d5` | perf: allocate the triangles of a mesh in one block |
| T53 | `d234c46` | perf: compile the camera shaders once for all cameras |
| T54 | `d3f3d61` | perf: build the local transform of a node in closed form |
| T55 | `4a62150` | perf: stop the dirty walk at a node that is already dirty |
| T56 | `9f1292d` | perf: compute the inverse and the decomposition of a part only when used |
| T57 | `b8eaa4e` | perf: sort the triangles of a small part with an insertion sort |
| T58 | `88e13f5` | perf: merge identical vertices of a GPU mesh |

T51 to T58 come from the performance audit of 2026-10-10. Each commit message gives its measurement. Build commit `6382f3d` comes before them and pins the Ebitengine fork freeze `fivehorizons-4`. T51 to T57 can go upstream, and T53 needs a version without the shader variants of the fork. T58 is fork only, because it changes the mesh path. Build commit `901da3a` comes after T58 and pins the Ebitengine fork freeze `fivehorizons-5`, commit `5d80ed8`.

T49 skips the depth of each triangle when the part draws its triangles unsorted: a part with the sort mode `TriangleSortModeNone`, which includes every solid part under the depth test, needs no distance from the camera, so `processVertices` no longer reads the triangle centres or computes the distance for it. On the CPU processing path of Five Horizons at 2150 m, the flat time of `processVertices` falls by about a quarter.

## Plan for the remaining upstream offers

Pace: one or two pull requests at a time. The maintainer discussed the method of these patches with Martin on Discord and merged all eleven pull requests.

| Wave | Patches | Offer | Issue first | Before sending |
| --- | --- | --- | --- | --- |
| 1 | T45, the fog skip | One pull request | No | Done: merged as #48. |
| 1 | T37, T38, T40, T41, the deprecated API | One pull request | No | Done: merged as #47. |
| 1 | T50, the glTF texture index | One pull request | No | Done: merged as #49. |
| 2 | T49, the unsorted triangle depth | One pull request after T21 and T22 | No | It waits for T21 and T22. Then measure it alone with the stock scene. |
| 2 | T15 to T20, the vertex pass and the indexed output | An issue, then a series of pull requests | Yes | Issue sent as #50. First pull request #51 with T15 and T18 is open. |
| 2 | T21 to T24, the packed triangles | A pull request after T15 to T20 | Yes | It adds `Mesh.UpdateTriangleData()`. `VertexBones` and `VertexWeights` are empty for a mesh without bones. |
| 2 | T25 and T26, `SetDepthTexture` and `ResetFrame` | One pull request | Yes | The API is public. It is independent now that B is upstream. |
| Hold | T39, the Kage rename | After upstream moves to Ebitengine v2.10 | A proposal to bump | Upstream is on v2.9.7. |
| Hold | T42, the Tetra3D option types | A proposal issue | Yes | It breaks the API. |
| Fork only | T27, T28, T29 to T36, T43, T44, T47 | None | Not applicable | They need the Ebitengine fork or are diagnostics. |

The numbers that the fork recorded for T45 (about -4.4% frame time, with T43 and T44, in `docs/devlog/079` of the game) and T49 (`docs/devlog/081`) come from the fork with its fork-only patches. They are not evidence for upstream.
