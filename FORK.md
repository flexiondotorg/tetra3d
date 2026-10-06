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
