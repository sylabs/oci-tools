// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/sylabs/oci-tools/pkg/sif"
)

// ImagePath returns the path to the image in the corpus with the specified name.
func (c *Corpus) ImagePath(name string) string {
	return filepath.Join(c.dir, "images", name)
}

// ImageIndex returns a v1.ImageIndex corresponding to the OCI Image Layout with the specified name
// in the corpus.
func (c *Corpus) ImageIndex(tb testing.TB, name string) v1.ImageIndex {
	tb.Helper()

	ii, err := layout.ImageIndexFromPath(c.ImagePath(name))
	if err != nil {
		tb.Fatalf("failed to get image index: %v", err)
	}

	return ii
}

// Image returns a v1.Image corresponding to the OCI Image Layout with the specified name in the
// corpus.
func (c *Corpus) Image(tb testing.TB, name string) v1.Image {
	tb.Helper()

	ii := c.ImageIndex(tb, name)

	img, err := ii.Image(v1.Hash{})
	if err != nil {
		tb.Fatalf("failed to get image from index: %v", err)
	}

	return img
}

// OCILayout returns a temporary OCI Image Layout for the test to use, populated from the OCI Image
// Layout with the specified name in the corpus. The directory is automatically removed when the
// test and all its subtests complete.
func (c *Corpus) OCILayout(tb testing.TB, name string) string {
	tb.Helper()

	lp, err := layout.Write(tb.TempDir(), c.ImageIndex(tb, name))
	if err != nil {
		tb.Fatalf("failed to write layout: %v", err)
	}

	return string(lp)
}

// SIF returns a temporary SIF for the test to use, populated from the OCI Image Layout with the
// specified name in the corpus. The SIF is automatically removed when the test and all its
// subtests complete.
func (c *Corpus) SIF(tb testing.TB, name string, opt ...sif.WriteOpt) string {
	tb.Helper()

	path := filepath.Join(tb.TempDir(), "image.sif")

	if err := sif.Write(path, c.ImageIndex(tb, name), opt...); err != nil {
		tb.Fatalf("failed to write SIF: %v", err)
	}

	return path
}
