// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sylabs/oci-tools/pkg/instrumented"
	"github.com/sylabs/oci-tools/pkg/ociplatform"
	ocisif "github.com/sylabs/oci-tools/pkg/sif"
	"github.com/sylabs/sif/v2/pkg/sif"
)

// sifSourceSink is used to retrieve/write images and indexes from/to a SIF file.
type sifSourceSink struct {
	ofi  *ocisif.OCIFileImage
	opts options
}

var _ SourceSink = &sifSourceSink{}

func handleOptionsSIF(opts ...Option) (*sifSourceSink, error) {
	ss := sifSourceSink{
		opts: options{},
	}
	for _, opt := range opts {
		if err := opt(&ss.opts); err != nil {
			return nil, err
		}
	}

	return &ss, nil
}

// SIFFromPath returns a sifSourceSink backed by an existing SIF file at src.
func SIFFromPath(src string, opts ...Option) (SourceSink, error) {
	s, err := handleOptionsSIF(opts...)
	if err != nil {
		return nil, err
	}

	fi, err := sif.LoadContainerFromPath(src)
	if err != nil {
		return nil, err
	}

	s.ofi, err = ocisif.FromFileImage(fi)
	if err != nil {
		return nil, err
	}

	return s, err
}

// SIFEmpty will create a new, empty SIF file at dst, with a specified capacity
// of descriptors, and return a sifSourceSink that can be used to write/read
// to/from it.
func SIFEmpty(dst string, descriptors int64, opts ...Option) (SourceSink, error) {
	s, err := handleOptionsSIF(opts...)
	if err != nil {
		return nil, err
	}
	if err := ocisif.Write(dst, empty.Index, ocisif.OptWriteWithSpareDescriptorCapacity(descriptors)); err != nil {
		return nil, err
	}

	fi, err := sif.LoadContainerFromPath(dst)
	if err != nil {
		return nil, err
	}
	s.ofi, err = ocisif.FromFileImage(fi)
	if err != nil {
		return nil, err
	}
	return s, nil
}

var _ Descriptor = &sifDescriptor{}

// sifDescriptor wraps a v1.Descriptor, providing methods to access the image or
// index to which it pertains, and the associated manifest, from an underlying
// SIF file.
type sifDescriptor struct {
	descriptor v1.Descriptor
	Manifest   []byte

	ofi    *ocisif.OCIFileImage
	parent *v1.Hash // digest of parent index if descriptor is not referenced in the RootIndex directly.

	instrumentationLogger *slog.Logger
}

// RawManifest returns the manifest of the image or index described by this
// descriptor.
func (d *sifDescriptor) RawManifest() ([]byte, error) {
	return d.Manifest, nil
}

// MediaType returns the types.MediaType of this descriptor.
func (d *sifDescriptor) MediaType() types.MediaType {
	return d.descriptor.MediaType
}

// Image returns a v1.Image directly if the descriptor is associated with an
// OCI image, or an image for the local platform if the descriptor is
// associated with an OCI ImageIndex.
func (d *sifDescriptor) Image() (v1.Image, error) {
	switch {
	case d.descriptor.MediaType.IsImage():
		var (
			img v1.Image
			err error
		)
		if d.parent == nil {
			img, err = d.ofi.Image(match.Digests(d.descriptor.Digest))
			if err != nil {
				return nil, err
			}
		} else {
			ii, err := d.ofi.Index(match.Digests(*d.parent))
			if err != nil {
				return nil, err
			}
			img, err = ii.Image(d.descriptor.Digest)
			if err != nil {
				return nil, err
			}
		}
		if d.instrumentationLogger != nil {
			return instrumented.Image(img, d.instrumentationLogger)
		}
		return img, nil

	case d.descriptor.MediaType.IsIndex():
		ii, err := d.ofi.Index(match.Digests(d.descriptor.Digest))
		if err != nil {
			return nil, err
		}
		p := ociplatform.DefaultPlatform()
		ims, err := partial.FindImages(ii, ociplatform.Matcher(p))
		if err != nil {
			return nil, err
		}
		if n := len(ims); n == 0 {
			return nil, ErrNoManifest
		} else if n > 1 {
			return nil, ErrMultipleManifests
		}
		if d.instrumentationLogger != nil {
			return instrumented.Image(ims[0], d.instrumentationLogger)
		}
		return ims[0], nil

	default:
		return nil, ErrUnsupportedMediaType
	}
}

// ImageIndex returns a v1.ImageIndex if the descriptor is associated with
// an OCI ImageIndex.
func (d *sifDescriptor) ImageIndex() (v1.ImageIndex, error) {
	if !d.descriptor.MediaType.IsIndex() {
		return nil, ErrUnsupportedMediaType
	}
	ii, err := d.ofi.Index(match.Digests(d.descriptor.Digest))
	if err != nil {
		return nil, err
	}
	if d.instrumentationLogger != nil {
		return instrumented.Index(ii, d.instrumentationLogger)
	}
	return ii, err
}

// getMatcher returns a Matcher that selects descriptors from an OCI layout
// index.json according to the getOpts provided.
func getMatcher(o getOpts) match.Matcher {
	return func(desc v1.Descriptor) bool {
		// Specified digest must match if provided.
		if o.digest != nil && desc.Digest != *o.digest {
			return false
		}

		if o.reference != nil {
			// Specified reference must match if provided.
			if desc.Annotations != nil || desc.Annotations[imagespec.AnnotationRefName] != o.reference.Name() {
				return false
			}
		} else {
			// Otherwise, no ref.name annotation is set.
			if desc.Annotations != nil && desc.Annotations[imagespec.AnnotationRefName] != "" {
				return false
			}
		}

		// If desc is an image, then must satisfy platform if specified.
		if o.platform != nil && desc.MediaType.IsImage() {
			return desc.Platform != nil && ociplatform.DescriptorSatisfies(desc, *o.platform)
		}
		return true
	}
}

// Get will find an image or index in the SIF file that matches the requirements
// specified by opts. If GetWithPlatform is specified then the Descriptor
// returned will always be an image that satisfies the platform. Otherwise, the
// Descriptor returned can be an image or an index.
func (o *sifSourceSink) Get(_ context.Context, opts ...GetOpt) (Descriptor, error) {
	gOpts := getOpts{}
	for _, opt := range opts {
		if err := opt(&gOpts); err != nil {
			return nil, err
		}
	}

	ds, err := o.ofi.FindManifests(getMatcher(gOpts))
	if err != nil {
		return nil, err
	}
	if len(ds) == 0 {
		return nil, ErrNoManifest
	}
	if len(ds) > 1 {
		return nil, ErrMultipleManifests
	}

	mt := ds[0].MediaType
	switch {
	case mt.IsImage():
		img, err := o.ofi.Image(match.Digests(ds[0].Digest))
		if err != nil {
			return nil, err
		}
		if gOpts.platform != nil {
			if err := ociplatform.EnsureImageSatisfies(img, *gOpts.platform); err != nil {
				return nil, err
			}
		}
		mf, err := img.RawManifest()
		if err != nil {
			return nil, err
		}
		return &sifDescriptor{
			descriptor:            ds[0],
			Manifest:              mf,
			ofi:                   o.ofi,
			instrumentationLogger: o.opts.instrumentationLogger,
		}, nil
	case mt.IsIndex():
		ii, err := o.ofi.Index(match.Digests(ds[0].Digest))
		if err != nil {
			return nil, err
		}
		// Platform wasn't requested - return the index itself.
		if gOpts.platform == nil {
			mf, err := ii.RawManifest()
			if err != nil {
				return nil, err
			}
			return &sifDescriptor{
				descriptor:            ds[0],
				Manifest:              mf,
				ofi:                   o.ofi,
				instrumentationLogger: o.opts.instrumentationLogger,
			}, nil
		}
		// Platform was requested - find an image in the index.
		return o.imageFromIndex(ii, gOpts.platform)
	default:
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedMediaType, mt)
	}
}

func (o *sifSourceSink) imageFromIndex(ii v1.ImageIndex, p *v1.Platform) (Descriptor, error) {
	iiDigest, err := ii.Digest()
	if err != nil {
		return nil, err
	}
	ims, err := partial.FindImages(ii, ociplatform.Matcher(p))
	if err != nil {
		return nil, err
	}
	if n := len(ims); n == 0 {
		return nil, ErrNoManifest
	} else if n > 1 {
		return nil, ErrMultipleManifests
	}
	d, err := partial.Descriptor(ims[0])
	if err != nil {
		return nil, err
	}
	mf, err := ims[0].RawManifest()
	if err != nil {
		return nil, err
	}
	return &sifDescriptor{
		descriptor: *d,
		Manifest:   mf,
		ofi:        o.ofi,
		parent:     &iiDigest, // must  be able to traverse index.json -> parent index -> image in OCIDescriptor.Image()
	}, nil
}

// Write will append an image or index w to the SIF file associated with the
// sifSourceSink.
func (o *sifSourceSink) Write(_ context.Context, w Writable, opts ...WriteOpt) error {
	wOpts := writeOpts{}
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return err
		}
	}

	appendOpts := []ocisif.AppendOpt{}
	if wOpts.reference != nil {
		appendOpts = append(appendOpts, ocisif.OptAppendReference(wOpts.reference))
	}

	if img, ok := w.(v1.Image); ok {
		return o.ofi.AppendImage(img, appendOpts...)
	}

	if ii, ok := w.(v1.ImageIndex); ok {
		return o.ofi.AppendIndex(ii, appendOpts...)
	}

	return ErrUnsupportedMediaType
}

// NumDescriptorsForImage returns the number of descriptors required to store img.
func NumDescriptorsForImage(img v1.Image) (int64, error) {
	ls, err := img.Layers()
	if err != nil {
		return 0, err
	}

	return int64(len(ls) + 2), nil
}

// NumDescriptorsForIndex returns the number of descriptors required to store ii.
func NumDescriptorsForIndex(ii v1.ImageIndex) (int64, error) {
	index, err := ii.IndexManifest()
	if err != nil {
		return 0, err
	}

	var count int64

	for _, desc := range index.Manifests {
		//nolint:exhaustive // Exhaustive cases not appropriate.
		switch desc.MediaType {
		case types.DockerManifestList, types.OCIImageIndex:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return 0, err
			}

			n, err := NumDescriptorsForIndex(ii)
			if err != nil {
				return 0, err
			}

			count += n

		case types.DockerManifestSchema2, types.OCIManifestSchema1:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return 0, err
			}

			n, err := NumDescriptorsForImage(img)
			if err != nil {
				return 0, err
			}

			count += n

		default:
			count++
		}
	}

	return count + 1, nil
}

var (
	errSIFBlobNoDigest  = errors.New("a digest must be provided to get a blob")
	errSIFBlobReference = errors.New("a reference cannot be provided when getting a blob")
	errSIFBlobPlatform  = errors.New("a platform cannot be provided when getting a blob")
)

// Blob returns an io.Readcloser for the content of the blob with a digest
// specified using the GetWithDigest option.
func (o *sifSourceSink) Blob(_ context.Context, opts ...GetOpt) (io.ReadCloser, error) {
	gOpts := getOpts{}
	for _, opt := range opts {
		if err := opt(&gOpts); err != nil {
			return nil, err
		}
	}

	if gOpts.reference != nil {
		return nil, errSIFBlobReference
	}
	if gOpts.platform != nil {
		return nil, errSIFBlobPlatform
	}
	if gOpts.digest == nil {
		return nil, errSIFBlobNoDigest
	}

	return o.ofi.Blob(*gOpts.digest)
}
