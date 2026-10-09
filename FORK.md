# Tetra3D fork for Five Horizons

This branch is a fork of [Tetra3D](https://github.com/SolarLune/tetra3d) for the game Five Horizons. The readme of the upstream project is `readme.md`.

- Upstream base: commit `6c1461c8f559d336e5fdadfb658f64f2a9dce453` on `main` (2026-08-18), 8 commits after `v0.18.0`.
- Branch: `fivehorizons`, one commit for each patch, on the upstream base. A rebase rewrites the branch. Each freeze is a tag.
- Licence: see `LICENSE`, unchanged from upstream.

## Patch order

1. T0: the pending upstream pull requests, #38 (the per-frame costs of `ProcessVertices`) and #40 (the stable counting sort), in their submitted form.
2. T1: general fixes and optimisations that keep the public API.
3. T2: optimisations and small additions that change an API or a behaviour.
4. T3: fork-only patches, for example the opt-in vertex counter behind the `vertexcount` build tag.
5. T4: the mesh path (GPU-resident meshes, instanced draws, and a hardware depth buffer) and the depth test inside the parts of the sorted path.

T0 to T3 build against stock Ebitengine. T4 needs the Ebitengine fork [`flexiondotorg/ebiten`](https://github.com/flexiondotorg/ebiten), branch `fivehorizons`, so `go.mod` has a `replace` line for it from the first T4 patch. The patches of T0 to T2 are offered upstream one at a time.

T46, the one pass of the mesh path (`Camera.GPUMeshOnePass` and `Camera.AfterMeshColour`), is a T4 patch: the parts draw their colour once with no depth pass, and a full-screen draw resolves the hardware depth of the colour texture into the depth texture. It needs `DrawTrianglesShaderOptions.ImageDepth` and `ebiten.IsDepthSourceSupported` of the Ebitengine fork.

T47, the hardware depth of the colour texture (`Camera.HardwareDepth`), is a T4 patch: `Render` draws the mesh path in one pass, runs `Camera.AfterMeshColour`, and draws each part of the sorted path once into the colour texture with the hardware depth test. Solid parts test and write the depth, alpha-clip parts also discard, and transparent parts test without a write, back to front. There is no depth texture, no `depthIntermediate`, no clear, no copy, and no resolve. It needs `DrawTrianglesShaderOptions.DepthReadOnly` of the Ebitengine fork, which also maps the clip z of a vertex function to a depth from 0 to 1 on OpenGL, so the resolve of T46 decodes that range.

T48 removes the one pass of T46 as a mode of its own, a T4 patch: `Camera.GPUMeshOnePass`, the depth resolve and its shader go, and the colour shaders of the mesh path with no gate now belong to `Camera.HardwareDepth`, which keeps `Camera.AfterMeshColour`. Without `HardwareDepth`, the mesh path draws its depth pass as before.

T49 skips the depth of each triangle when the part draws its triangles unsorted, a T1 patch: a part with the sort mode `TriangleSortModeNone`, which includes every solid part under the depth test, needs no distance from the camera, so `processVertices` no longer reads the triangle centres or computes the distance for it. On the CPU processing path of Five Horizons at 2150 m, the flat time of `processVertices` falls by about a quarter.
