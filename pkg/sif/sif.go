// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// OCIFileImage represents a Singularity Image Format (SIF) file containing OCI
// artifacts.
type OCIFileImage struct {
	sif *sif.FileImage
}

// FromFileImage constructs an extended oci-tools OCIFileImage, with OCI
// specific functionality, from a generic sif.FileImage.
func FromFileImage(fi *sif.FileImage) (*OCIFileImage, error) {
	f := &OCIFileImage{sif: fi}
	return f, nil
}

// Blob returns a ReadCloser that reads the blob with the supplied digest.
func (f *OCIFileImage) Blob(h v1.Hash) (io.ReadCloser, error) {
	d, err := f.sif.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return nil, err
	}

	return io.NopCloser(d.GetReader()), nil
}

// Bytes returns the bytes of the blob with the supplied digest.
func (f *OCIFileImage) Bytes(h v1.Hash) ([]byte, error) {
	d, err := f.sif.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return nil, err
	}

	return d.GetData()
}

// Offset returns the offset within the SIF image of the blob with the supplied digest.
func (f *OCIFileImage) Offset(h v1.Hash) (int64, error) {
	d, err := f.sif.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return 0, err
	}

	return d.Offset(), nil
}

// FindManifests finds the manifests stored in f that are selected by m. If m is nil, all manifests
// are selected.
func (f *OCIFileImage) FindManifests(m match.Matcher, _ ...Option) ([]v1.Descriptor, error) {
	ri, err := f.RootIndex()
	if err != nil {
		return nil, err
	}

	return partial.FindManifests(ri, matchAllIfNil(m))
}
