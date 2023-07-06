// Copyright 2023 Sylabs Inc. All rights reserved.
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

	args := []string{"-a", pathA, "-b", pathB}
	args = append(args, diffArgs...)

	cmd := exec.Command("sqfsdiff", args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		tb.Log(string(out))
		tb.Errorf("sqfsdiff: %v", err)
	}
}

func Test_squashfsFromLayer(t *testing.T) {
	tests := []struct {
		name      string
		layer     v1.Layer
		converter string
		diffArgs  []string
	}{
		{
			name: "HelloWorldBlob_sqfstar",
			layer: testLayer(t, "hello-world-docker-v2-manifest", v1.Hash{
				Algorithm: "sha256",
				Hex:       "7050e35b49f5e348c4809f5eff915842962cb813f32062d3bbdd35c750dd7d01",
			}),
			converter: "sqfstar",
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
			converter: "tar2sqfs",
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			if _, err := exec.LookPath(tt.converter); errors.Is(err, exec.ErrNotFound) {
				t.Skip(err)
			}

			l, err := squashfsFromLayer(tt.layer, tt.converter, t.TempDir())
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
