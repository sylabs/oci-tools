// Copyright 2024 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sif

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// updateOpts accumulates update options.
type updateOpts struct {
	// tempDir is os.TempDir or user supplied value
	tempDir string
	// cacheDir created inside tempDir
	cacheDir string
}

// UpdateOpt are used to specify options to apply when updating a SIF.
type UpdateOpt func(*updateOpts) error

// OptUpdateTempDir sets the directory to use for temporary files. If not set, the
// directory returned by os.TempDir is used.
func OptUpdateTempDir(d string) UpdateOpt {
	return func(c *updateOpts) error {
		c.tempDir = d
		return nil
	}
}

// UpdateRootIndex modifies the SIF file associated with f so that it holds the
// content of ImageIndex ii. The RootIndex of the SIF is replaced with ii. Any
// blobs in the SIF that are not referenced in ii are removed from the SIF. Any
// blobs that are referenced in ii but not present in the SIF are added to the
// SIF.
//
// UpdateRootIndex may create one or more temporary files during the update
// process. By default, the directory returned by os.TempDir is used. To
// override this, consider using OptUpdateTmpDir.
func (f *OCIFileImage) UpdateRootIndex(ii v1.ImageIndex, opts ...UpdateOpt) error {
	uo := updateOpts{
		tempDir: os.TempDir(),
	}
	for _, opt := range opts {
		if err := opt(&uo); err != nil {
			return err
		}
	}
	defer func() {
		if uo.cacheDir != "" {
			os.RemoveAll(uo.cacheDir)
		}
	}()

	// If the existing OCI.RootIndex in the SIF matches ii, then there is nothing to do.
	sifRootIndex, err := f.RootIndex()
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
	sifBlobs, err := sifBlobs(f.sif)
	if err != nil {
		return err
	}

	// Cache all new blobs referenced by the new ImageIndex and its child
	// indices / images, which aren't already in the SIF. cachedblobs are new
	// things to add. keepBlobs already exist in the SIF and should be kept.
	cachedBlobs, keepBlobs, err := uo.cacheIndexBlobs(ii, sifBlobs)
	if err != nil {
		return err
	}

	// Compute the new RootIndex.
	ri, err := ii.RawManifest()
	if err != nil {
		return err
	}

	// Delete existing blobs from the SIF except those we want to keep.
	if err := f.sif.DeleteObjects(selectBlobsExcept(keepBlobs),
		sif.OptDeleteZero(true),
		sif.OptDeleteCompact(true),
	); err != nil {
		return err
	}

	// Write new (cached) blobs from ii into the SIF.
	for _, b := range cachedBlobs {
		rc, err := uo.readCacheBlob(b)
		if err != nil {
			return err
		}
		if err := f.WriteBlob(rc); err != nil {
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}

	// Write the new RootIndex into the SIF.
	return f.writeRootIndex(bytes.NewReader(ri))
}

// Update is a convenience function, for backward compatibility, which calls
// OCIFileImage.UpdateRootIndex against the sif.FileImage fi.
//
// Deprecated: Use OCIFileImage.UpdateRootIndex instead.
func Update(fi *sif.FileImage, ii v1.ImageIndex, opts ...UpdateOpt) error {
	f, err := FromFileImage(fi)
	if err != nil {
		return err
	}
	return f.UpdateRootIndex(ii, opts...)
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

// cacheIndexBlobs will cache all blobs referenced by ii, except those with
// digests specified in skip. The blobs will be cached to files in cacheDir,
// with filenames equal to their digest. The function returns two lists of blobs
// - those that were cached (in ii but not skip), and those that were skipped
// (in ii and skip).
func (uo *updateOpts) cacheIndexBlobs(ii v1.ImageIndex, skip []v1.Hash) ([]v1.Hash, []v1.Hash, error) {
	index, err := ii.IndexManifest()
	if err != nil {
		return nil, nil, err
	}

	cached := []v1.Hash{}
	skipped := []v1.Hash{}

	for _, desc := range index.Manifests {
		//nolint:exhaustive
		switch desc.MediaType {
		case types.DockerManifestList, types.OCIImageIndex:
			childIndex, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return nil, nil, err
			}
			// Cache children of this ImageIndex
			childCached, childSkipped, err := uo.cacheIndexBlobs(childIndex, skip)
			if err != nil {
				return nil, nil, err
			}
			cached = append(cached, childCached...)
			skipped = append(skipped, childSkipped...)
			// Cache the ImageIndex itself.
			if slices.Contains(skip, desc.Digest) {
				skipped = append(skipped, desc.Digest)
				continue
			}
			rm, err := childIndex.RawManifest()
			if err != nil {
				return nil, nil, err
			}
			rc := io.NopCloser(bytes.NewReader(rm))
			if err := uo.writeCacheBlob(rc, desc.Digest); err != nil {
				return nil, nil, err
			}
			cached = append(cached, desc.Digest)

		case types.DockerManifestSchema2, types.OCIManifestSchema1:
			childImage, err := ii.Image(desc.Digest)
			if err != nil {
				return nil, nil, err
			}
			childCached, childSkipped, err := uo.cacheImageBlobs(childImage, skip)
			if err != nil {
				return nil, nil, err
			}
			cached = append(cached, childCached...)
			skipped = append(skipped, childSkipped...)

		default:
			return nil, nil, errUnexpectedMediaType
		}
	}
	return cached, skipped, nil
}

// cacheImageBlobs will cache all blobs referenced by im, except those with
// digests specified in skip. The blobs will be cached to files in cacheDir,
// with filenames equal to their digest. The function returns lists of blobs
// that were cached (in ii but not skip), and those that were skipped (in ii and
// skipDigests).
func (uo *updateOpts) cacheImageBlobs(im v1.Image, skip []v1.Hash) ([]v1.Hash, []v1.Hash, error) {
	cached := []v1.Hash{}
	skipped := []v1.Hash{}

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

		if slices.Contains(skip, ld) {
			skipped = append(skipped, ld)
			continue
		}

		rc, err := l.Compressed()
		if err != nil {
			return nil, nil, err
		}
		if err := uo.writeCacheBlob(rc, ld); err != nil {
			return nil, nil, err
		}
		cached = append(cached, ld)
	}

	// Cache image config.
	mf, err := im.Manifest()
	if err != nil {
		return nil, nil, err
	}
	if slices.Contains(skip, mf.Config.Digest) {
		skipped = append(skipped, mf.Config.Digest)
	} else {
		c, err := im.RawConfigFile()
		if err != nil {
			return nil, nil, err
		}
		rc := io.NopCloser(bytes.NewReader(c))
		if err := uo.writeCacheBlob(rc, mf.Config.Digest); err != nil {
			return nil, nil, err
		}
		cached = append(cached, mf.Config.Digest)
	}

	// Cache image manifest itself.
	id, err := im.Digest()
	if err != nil {
		return nil, nil, err
	}
	if slices.Contains(skip, id) {
		skipped = append(skipped, id)
		return cached, skipped, nil
	}
	rm, err := im.RawManifest()
	if err != nil {
		return nil, nil, err
	}
	rc := io.NopCloser(bytes.NewReader(rm))
	if err := uo.writeCacheBlob(rc, id); err != nil {
		return nil, nil, err
	}
	cached = append(cached, id)

	return cached, skipped, nil
}

// writeCacheBlob writes blob content from rc into a cache directory with
// filename equal to specified digest.
func (uo *updateOpts) writeCacheBlob(rc io.ReadCloser, digest v1.Hash) error {
	if uo.cacheDir == "" {
		var err error
		if uo.cacheDir, err = os.MkdirTemp(uo.tempDir, ""); err != nil {
			return err
		}
	}

	path := filepath.Join(uo.cacheDir, digest.String())
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

var errNoCacheDir = errors.New("cacheDir not set")

// readCacheBlob returns a ReadCloser that will read blob content from the cache
// directory with filename equal to specified digest.
func (uo *updateOpts) readCacheBlob(digest v1.Hash) (io.ReadCloser, error) {
	if uo.cacheDir == "" {
		return nil, errNoCacheDir
	}
	path := filepath.Join(uo.cacheDir, digest.String())
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// selectBlobsExcept selects all OCI.RootIndex/OCI.Blob descriptors, except those with
// digests listed in keep.
func selectBlobsExcept(keep []v1.Hash) sif.DescriptorSelectorFunc {
	return func(d sif.Descriptor) (bool, error) {
		if h, err := d.OCIBlobDigest(); err == nil && !slices.Contains(keep, h) {
			return true, nil
		}
		return false, nil
	}
}

// appendOpts accumulates append options.
type appendOpts struct {
	tempDir string
	ref     name.Reference
}

// AppendOpt are used to specify options to apply when appending to a SIF.
type AppendOpt func(*appendOpts) error

// OptAppendTempDir sets the directory to use for temporary files. If not set, the
// directory returned by os.TempDir is used.
func OptAppendTempDir(d string) AppendOpt {
	return func(c *appendOpts) error {
		c.tempDir = d
		return nil
	}
}

// OptAppendReference sets the reference to be set for the appended item in the
// RootIndex. The reference is added as an `org.opencontainers.image.ref.name`
// in the RootIndex.
func OptAppendReference(r name.Reference) AppendOpt {
	return func(c *appendOpts) error {
		c.ref = r
		return nil
	}
}

// AppendImage appends an image to the SIF f, updating the RootIndex to
// reference it.
func (f *OCIFileImage) AppendImage(img v1.Image, opts ...AppendOpt) error {
	return f.append(img, opts...)
}

// AppendIndex appends an index to the SIF f, updating the RootIndex to
// reference it.
func (f *OCIFileImage) AppendIndex(ii v1.ImageIndex, opts ...AppendOpt) error {
	return f.append(ii, opts...)
}

func (f *OCIFileImage) append(add mutate.Appendable, opts ...AppendOpt) error {
	ao := appendOpts{
		tempDir: os.TempDir(),
	}
	for _, opt := range opts {
		if err := opt(&ao); err != nil {
			return err
		}
	}

	ri, err := f.RootIndex()
	if err != nil {
		return err
	}

	ri, err = appendToIndex(ri, add, ao)
	if err != nil {
		return err
	}

	return f.UpdateRootIndex(ri, OptUpdateTempDir(ao.tempDir))
}

func appendToIndex(base v1.ImageIndex, add mutate.Appendable, ao appendOpts) (v1.ImageIndex, error) {
	ia := mutate.IndexAddendum{Add: add}

	var err error
	if ao.ref != nil {
		base, err = removeRefAnnotation(base, ao.ref)
		if err != nil {
			return nil, err
		}

		d, err := partial.Descriptor(add)
		if err != nil {
			return nil, err
		}
		if d.Annotations != nil {
			ia.Annotations = maps.Clone(d.Annotations)
		} else {
			ia.Annotations = make(map[string]string)
		}
		ia.Annotations[imagespec.AnnotationRefName] = ao.ref.Name()
	}

	return mutate.AppendManifests(base, ia), nil
}

// removeRefAnnotation removes an existing "org.opencontainers.image.ref.name"
// annotation with the provided value from the ImageIndex ii.
func removeRefAnnotation(ii v1.ImageIndex, ref name.Reference) (v1.ImageIndex, error) {
	return editManifestDescriptors(
		ii,
		match.Name(ref.Name()),
		func(desc v1.Descriptor) v1.Descriptor {
			delete(desc.Annotations, imagespec.AnnotationRefName)
			return desc
		},
	)
}

// RemoveBlob removes a blob from the SIF f, without modifying the rootIndex.
func (f *OCIFileImage) RemoveBlob(hash v1.Hash) error {
	return f.sif.DeleteObjects(sif.WithOCIBlobDigest(hash),
		sif.OptDeleteZero(true),
		sif.OptDeleteCompact(true))
}

// RemoveManifests modifies the SIF file associated with f so that its RootIndex
// no longer holds manifests selected by matcher. Any blobs in the SIF that are
// no longer referenced are removed from the SIF.
func (f *OCIFileImage) RemoveManifests(matcher match.Matcher) error {
	ri, err := f.RootIndex()
	if err != nil {
		return err
	}
	return f.UpdateRootIndex(mutate.RemoveManifests(ri, matcher))
}

// ReplaceImage writes img to the SIF, replacing any existing manifest that is
// selected by the matcher. Any blobs in the SIF that are no longer referenced
// are removed from the SIF.
func (f *OCIFileImage) ReplaceImage(img v1.Image, matcher match.Matcher, opts ...AppendOpt) error {
	return f.replace(img, matcher, opts...)
}

// ReplaceIndex writes ii to the SIF, replacing any existing manifest that is
// selected by the matcher. Any blobs in the SIF that are no longer referenced
// are removed from the SIF.
func (f *OCIFileImage) ReplaceIndex(ii v1.ImageIndex, matcher match.Matcher, opts ...AppendOpt) error {
	return f.replace(ii, matcher, opts...)
}

func (f *OCIFileImage) replace(add mutate.Appendable, matcher match.Matcher, opts ...AppendOpt) error {
	ao := appendOpts{
		tempDir: os.TempDir(),
	}
	for _, opt := range opts {
		if err := opt(&ao); err != nil {
			return err
		}
	}

	ri, err := f.RootIndex()
	if err != nil {
		return err
	}

	ri = mutate.RemoveManifests(ri, matcher)

	ri, err = appendToIndex(ri, add, ao)
	if err != nil {
		return err
	}

	return f.UpdateRootIndex(ri, OptUpdateTempDir(ao.tempDir))
}
