module github.com/solarlune/tetra3d

go 1.26.0

require (
	github.com/hajimehoshi/ebiten/v2 v2.9.7
	github.com/qmuntal/gltf v0.28.1-0.20260527150304-a48a560f8f1c
	github.com/tanema/gween v0.0.0-20250522035225-e874ee3ae01a
	golang.org/x/image v0.46.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

// This fork calls the mesh path of the Ebitengine fork (ebiten.NewMesh), so
// build and test it against that fork, as the game does.
replace github.com/hajimehoshi/ebiten/v2 => github.com/flexiondotorg/ebiten/v2 v2.0.0-20261006021447-938520fcbc59
