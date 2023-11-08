// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// fileImage represents a Singularity Image Format (SIF) file containing OCI artifacts.
type fileImage struct {
	*sif.FileImage
}

// Blob returns a ReadCloser that reads the blob with the supplied digest.
func (f *fileImage) Blob(h v1.Hash) (io.ReadCloser, error) {
	d, err := f.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return nil, err
	}

	return io.NopCloser(d.GetReader()), nil
}

// Bytes returns the bytes of the blob with the supplied digest.
func (f *fileImage) Bytes(h v1.Hash) ([]byte, error) {
	d, err := f.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return nil, err
	}

	return d.GetData()
}

// Offset returns the offset within the SIF image of the blob with the supplied digest.
func (f *fileImage) Offset(h v1.Hash) (int64, error) {
	d, err := f.GetDescriptor(sif.WithOCIBlobDigest(h))
	if err != nil {
		return 0, err
	}

	return d.Offset(), nil
}
