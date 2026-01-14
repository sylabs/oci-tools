// Copyright 2023-2026 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sebdah/goldie/v2"
)

func testLayer(tb testing.TB, name string, digest v1.Hash) v1.Layer {
	tb.Helper()

	img := corpus.Image(tb, name)

	l, err := img.LayerByDigest(digest)
	if err != nil {
		tb.Fatal(err)
	}

	return l
}

// diffSquashFS compares two SquashFS images, and reports a test error if a difference is found.
// Two images are considered equal if each directory contains the same entries, symlink with the
// same paths have the same targets, device nodes the same device number and files the same size
// and contents.
func diffSquashFS(tb testing.TB, pathA, pathB string, diffArgs ...string) {
	tb.Helper()

	args := make([]string, 0, 4+len(diffArgs))
	args = append(args, "-a", pathA, "-b", pathB)
	args = append(args, diffArgs...)

	cmd := exec.CommandContext(tb.Context(), "sqfsdiff", args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		tb.Log(string(out))
		tb.Errorf("sqfsdiff: %v", err)
	}
}

func Test_SquashfsLayer(t *testing.T) {
	squashImage, err := Squash(corpus.Image(t, "root-dir-entry"))
	if err != nil {
		t.Fatal(err)
	}

	squashLayers, err := squashImage.Layers()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name              string
		layer             v1.Layer
		converter         string
		noConvertWhiteout bool
		diffArgs          []string
	}{
		// Simple image that contains a root directory entry "./", which caused a bug when the
		// image was first "squashed" and then converted to SquashFS.
		{
			name:      "RootDirEntry",
			layer:     squashLayers[0],
			converter: "sqfstar",
			// Some versions of squashfs-tools do not implement '-root-uid'/'-root-gid', so ignore
			// differences in ownership.
			diffArgs: []string{"--no-owner"},
		},
		// HelloWorld layer contains no whiteouts - conversion should have no effect on output.
		{
			name: "HelloWorldBlob_sqfstar",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			converter:         "sqfstar",
			noConvertWhiteout: false,
			// Some versions of squashfs-tools do not implement '-root-uid'/'-root-gid', so ignore
			// differences in ownership.
			diffArgs: []string{"--no-owner"},
		},
		{
			name: "HelloWorldBlob_tar2sqfs",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			converter:         "tar2sqfs",
			noConvertWhiteout: false,
		},
		{
			name: "HelloWorldBlob_sqfstar_SkipWhiteoutConversion",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			converter:         "sqfstar",
			noConvertWhiteout: true,
			// Some versions of squashfs-tools do not implement '-root-uid'/'-root-gid', so ignore
			// differences in ownership.
			diffArgs: []string{"--no-owner"},
		},
		{
			name: "HelloWorldBlob_tar2sqfs_SkipWhiteoutConversion",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			converter:         "tar2sqfs",
			noConvertWhiteout: true,
		},
		// AUFS layer contains whiteouts. Should be converted to overlayfs form when noConvertWhiteout = false.
		//
		//    Original (AUFS)
		//		All regular files.
		//
		//        [drwxr-xr-x]  .
		//			├── [drwxr-xr-x]  dir
		//			│   └── [-rw-r--r--]  .wh..wh..opq
		//			└── [-rw-r--r--]  .wh.file
		//
		//    Converted (OverlayFS)
		//		.wh.file becomes file as a char 0:0 device
		// 		dir/.wh..wh..opq becomes trusted.overlay.opaque="y" xattr on dir
		//
		//			[drwxr-xr-x]  .
		//			├── [drwxr-xr-x]  dir
		//			└── [crw-r--r--]  file
		//
		{
			name: "AUFSBlob_sqfstar",
			layer: testLayer(t, "aufs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "da55812559dec81445c289c3832cee4a2f725b15aeb258791640185c3126b2bf",
			}),
			converter:         "sqfstar",
			noConvertWhiteout: false,
			// Some versions of squashfs-tools do not implement '-root-uid'/'-root-gid', so ignore
			// differences in ownership.
			diffArgs: []string{"--no-owner"},
		},
		{
			name: "AUFSBlob_tar2sqfs",
			layer: testLayer(t, "aufs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "da55812559dec81445c289c3832cee4a2f725b15aeb258791640185c3126b2bf",
			}),
			converter:         "tar2sqfs",
			noConvertWhiteout: false,
		},
		{
			name: "AUFSBlob_sqfstar_SkipWhiteoutConversion",
			layer: testLayer(t, "aufs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "da55812559dec81445c289c3832cee4a2f725b15aeb258791640185c3126b2bf",
			}),
			converter:         "sqfstar",
			noConvertWhiteout: true,
			// Some versions of squashfs-tools do not implement '-root-uid'/'-root-gid', so ignore
			// differences in ownership.
			diffArgs: []string{"--no-owner"},
		},
		{
			name: "AUFSBlob_tar2sqfs_SkipWhiteoutConversion",
			layer: testLayer(t, "aufs-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "da55812559dec81445c289c3832cee4a2f725b15aeb258791640185c3126b2bf",
			}),
			converter:         "tar2sqfs",
			noConvertWhiteout: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := exec.LookPath(tt.converter); errors.Is(err, exec.ErrNotFound) {
				t.Skip(err)
			}

			l, err := SquashfsLayer(tt.layer, t.TempDir(),
				OptSquashfsLayerConverter(tt.converter),
				OptSquashfsSkipWhiteoutConversion(tt.noConvertWhiteout),
			)
			if err != nil {
				t.Fatal(err)
			}

			rc, err := l.Uncompressed()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { rc.Close() })

			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(t.TempDir(), "layer.sqfs")

			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}

			g := goldie.New(t, goldie.WithTestNameForDir(true))

			// // Un-comment to regenerate golden files...
			// b, err := os.ReadFile(path)
			// if err != nil {
			// 	t.Fatal(err)
			// }
			// g.Assert(t, tt.name, b)

			diffSquashFS(t, path, g.GoldenFileName(t, tt.name), tt.diffArgs...)
		})
	}
}
