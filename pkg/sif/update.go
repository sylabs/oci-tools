// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// updateOpts accumulates update options.
type updateOpts struct {
	tempDir string
}

// UpdateOpt are used to specify options to apply when updating a SIF.
type UpdateOpt func(*updateOpts) error

// OptUpdateTempDir sets the directory to use for temporary files. If not set, the
// directory returned by TempDir is used.
func OptTarTempDir(d string) UpdateOpt {
	return func(c *updateOpts) error {
		c.tempDir = d
		return nil
	}
}

// Update modifies the SIF file associated with fi so that it holds the content
// of ImageIndex ii. Any blobs in the SIF that are not referenced in ii are
// removed from the SIF. Any blobs that are referenced in ii but not present in
// the SIF are added to the SIF. The RootIndex of the SIF is replaced with ii.
//
// Update may create one or more temporary files during the update process. By
// default, the directory returned by TempDir is used. To override this,
// consider using OptUpdateTmpDir.
func Update(fi *sif.FileImage, ii v1.ImageIndex, opts ...UpdateOpt) error {
	uo := updateOpts{}
	for _, opt := range opts {
		if err := opt(&uo); err != nil {
			return err
		}
	}

	// If the existing OCI.RootIndex in the SIF matches ii, then there is nothing to do.
	sifRootIndex, err := ImageIndexFromFileImage(fi)
	if err != nil {
		return err
	}
	sifRootDigest, err := sifRootIndex.Digest()
	if err != nil {
		return err
	}
	newRootDigest, err := ii.Digest()
	if err != nil {
		return err
	}
	if sifRootDigest == newRootDigest {
		return nil
	}

	// Get a list of all existing OCI.Blob digests in the SIF
	sifBlobs, err := sifBlobs(fi)
	if err != nil {
		return err
	}

	// Cache all new blobs referenced by the new ImageIndex and its child
	// indices / images, which aren't already in the SIF. cachedblobs are new
	// things to add. keepBlobs already exist in the SIF and should be kept.
	blobCache, err := os.MkdirTemp(uo.tempDir, "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(blobCache)
	cachedBlobs, keepBlobs, err := cacheIndexBlobs(ii, sifBlobs, blobCache)
	if err != nil {
		return err
	}

	// Compute the new RootIndex.
	ri, err := ii.RawManifest()
	if err != nil {
		return err
	}

	// Delete existing blobs from the SIF except those we want to keep.
	if err := deleteBlobsExcept(fi, keepBlobs); err != nil {
		return err
	}
	// Delete old RootIndex.
	if err := deleteRootIndex(fi); err != nil {
		return err
	}

	// Write new (cached) blobs from ii into the SIF.
	f := fileImage{fi}
	for _, b := range cachedBlobs {
		rc, err := readCacheBlob(b, blobCache)
		if err != nil {
			return err
		}
		if err := f.writeBlobToFileImage(rc, false); err != nil {
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}

	// Write the new RootIndex into the SIF.
	return f.writeBlobToFileImage(bytes.NewReader(ri), true)
}

// sifBlobs will return a list of digests for all OCI.Blob descriptors in fi.
func sifBlobs(fi *sif.FileImage) ([]v1.Hash, error) {
	descrs, err := fi.GetDescriptors(sif.WithDataType(sif.DataOCIBlob))
	if err != nil {
		return nil, err
	}
	sifBlobs := make([]v1.Hash, len(descrs))
	for i, d := range descrs {
		dDigest, err := d.OCIBlobDigest()
		if err != nil {
			return nil, err
		}
		sifBlobs[i] = dDigest
	}
	return sifBlobs, nil
}

// cacheIndexBlobs will cache all blobs referenced by ii, except those specified
// in skipDigests. The blobs will be cached as files in cacheDir, with filenames
// equal to their digest. The function returns lists of blobs that were cached
// (in ii but not skipDigests), and those that were skipped (in ii and
// skipDigests).
func cacheIndexBlobs(ii v1.ImageIndex, skipDigests []v1.Hash, cacheDir string) (cached []v1.Hash, skipped []v1.Hash, err error) {
	index, err := ii.IndexManifest()
	if err != nil {
		return nil, nil, err
	}

	for _, desc := range index.Manifests {
		//nolint:exhaustive
		switch desc.MediaType {
		case types.DockerManifestList, types.OCIImageIndex:
			childIndex, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return nil, nil, err
			}
			// Cache children of this ImageIndex
			childCached, childSkipped, err := cacheIndexBlobs(childIndex, skipDigests, cacheDir)
			if err != nil {
				return nil, nil, err
			}
			cached = append(cached, childCached...)
			skipped = append(skipped, childSkipped...)
			// Cache ImageIndex itself.
			if slices.Contains(skipDigests, desc.Digest) {
				skipped = append(skipped, desc.Digest)
				continue
			}
			rm, err := childIndex.RawManifest()
			if err != nil {
				return nil, nil, err
			}
			rc := io.NopCloser(bytes.NewReader(rm))
			if err := writeCacheBlob(rc, desc.Digest, cacheDir); err != nil {
				return nil, nil, err
			}
			cached = append(cached, desc.Digest)

		case types.DockerManifestSchema2, types.OCIManifestSchema1:
			childImage, err := ii.Image(desc.Digest)
			if err != nil {
				return nil, nil, err
			}
			// Cache children of this image (layers, config)
			childCached, childSkipped, err := cacheImageBlobs(childImage, skipDigests, cacheDir)
			if err != nil {
				return nil, nil, err
			}
			cached = append(cached, childCached...)
			skipped = append(skipped, childSkipped...)
			// Cache image manifest itself.
			if slices.Contains(skipDigests, desc.Digest) {
				skipped = append(skipped, desc.Digest)
				continue
			}
			rm, err := childImage.RawManifest()
			if err != nil {
				return nil, nil, err
			}
			rc := io.NopCloser(bytes.NewReader(rm))
			if err := writeCacheBlob(rc, desc.Digest, cacheDir); err != nil {
				return nil, nil, err
			}
			cached = append(cached, desc.Digest)

		default:
			if slices.Contains(skipDigests, desc.Digest) {
				skipped = append(skipped, desc.Digest)
				continue
			}
			rc, err := blobFromIndex(ii, desc.Digest)
			if err != nil {
				return nil, nil, err
			}
			if err := writeCacheBlob(rc, desc.Digest, cacheDir); err != nil {
				return nil, nil, err
			}
			cached = append(cached, desc.Digest)
		}
	}
	return cached, skipped, nil
}

// cacheImageBlobs will cache all blobs referenced by im, except those specified
// in skipDigests. The blobs will be cached as files in cacheDir, with filenames
// equal to their digest. The function returns lists of blobs that were cached
// (in ii but not skipDigests), and those that were skipped (in ii and
// skipDigests).
func cacheImageBlobs(im v1.Image, skipDigests []v1.Hash, cacheDir string) (cached []v1.Hash, skipped []v1.Hash, err error) {
	// Cache layers first.
	layers, err := im.Layers()
	if err != nil {
		return nil, nil, err
	}
	for _, l := range layers {
		ld, err := l.Digest()
		if err != nil {
			return nil, nil, err
		}

		if slices.Contains(skipDigests, ld) {
			skipped = append(skipped, ld)
			continue
		}

		rc, err := l.Compressed()
		if err != nil {
			return nil, nil, err
		}
		if err := writeCacheBlob(rc, ld, cacheDir); err != nil {
			return nil, nil, err
		}
		cached = append(cached, ld)
	}

	// Cache image config.
	// Note - Manifest is cached from parent ImageIndex.
	mf, err := im.Manifest()
	if err != nil {
		return nil, nil, err
	}
	if slices.Contains(skipDigests, mf.Config.Digest) {
		skipped = append(skipped, mf.Config.Digest)
		return cached, skipped, nil
	}
	c, err := im.RawConfigFile()
	if err != nil {
		return nil, nil, err
	}
	rc := io.NopCloser(bytes.NewReader(c))
	if err := writeCacheBlob(rc, mf.Config.Digest, cacheDir); err != nil {
		return nil, nil, err
	}
	cached = append(cached, mf.Config.Digest)

	return cached, skipped, nil
}

// writeCacheBlob writes blob content from rc into tmpDir with filename equal to
// specified digest.
func writeCacheBlob(rc io.ReadCloser, digest v1.Hash, cacheDir string) error {
	path := filepath.Join(cacheDir, digest.String())
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, rc)
	if err != nil {
		return err
	}

	if err := rc.Close(); err != nil {
		return err
	}
	return nil
}

// readCacheBlob returns a ReadCloser that will read blob content from cacheDir
// with filename equal to specified digest.
func readCacheBlob(digest v1.Hash, cacheDir string) (io.ReadCloser, error) {
	path := filepath.Join(cacheDir, digest.String())
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// deleteBlobsExcept removes all OCI.Blob descriptors from fi, except those with
// digests listed in keepDigests.
func deleteBlobsExcept(fi *sif.FileImage, keepDigests []v1.Hash) error {
	descs, err := fi.GetDescriptors(sif.WithDataType(sif.DataOCIBlob))
	if err != nil {
		return err
	}
	for _, d := range descs {
		dd, err := d.OCIBlobDigest()
		if err != nil {
			return err
		}
		if slices.Contains(keepDigests, dd) {
			continue
		}
		if err := fi.DeleteObject(d.ID()); err != nil {
			return err
		}
	}
	return nil
}

// deleteRootIndex removes the RootIndex from a the SIF fi.
func deleteRootIndex(fi *sif.FileImage) error {
	desc, err := fi.GetDescriptor(sif.WithDataType(sif.DataOCIRootIndex))
	if err != nil {
		return err
	}
	return fi.DeleteObject(desc.ID())
}
