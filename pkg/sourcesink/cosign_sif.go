// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"bytes"
	"context"
	"errors"
	"log/slog"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	cosignoci "github.com/sigstore/cosign/v2/pkg/oci"
	cosignempty "github.com/sigstore/cosign/v2/pkg/oci/empty"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	cosignsignature "github.com/sigstore/cosign/v2/pkg/oci/signature"
	"github.com/sylabs/oci-tools/pkg/sif"
)

var _ SignedDescriptor = &sifDescriptor{}

// CosignImages checks for image manifests providing cosign signatures &
// attestations that are associated with the image or index with the descriptor
// If image manifests providing cosign signatures and / or attestations
// exist, then these images are returned in a name.Reference -> v1.Image map.
//
// If recursive is true, then if the descriptor is an index, we also check for
// signatures and attestations for each of its associated manifests.
//
// In the returned map, the images are referenced as '_cosign:<tag>', where
// <tag> matches the tag at src. The '_cosign' repository placeholder string
// is used instead of any original registry & repository names.
func (d *sifDescriptor) CosignImages(_ context.Context, recursive bool) ([]ReferencedImage, error) {
	csImgs := []ReferencedImage{}

	// Targets lists digests to check for the existence of associated signatures / images.
	targets := []v1.Hash{d.descriptor.Digest}

	if d.MediaType().IsIndex() && recursive {
		rmf, err := d.RawManifest()
		if err != nil {
			return nil, err
		}
		mf, err := v1.ParseIndexManifest(bytes.NewBuffer(rmf))
		if err != nil {
			return nil, err
		}
		for _, m := range mf.Manifests {
			targets = append(targets, m.Digest)
		}
	}

	for _, target := range targets {
		for _, suffix := range cosignSuffixes {
			csRef, err := CosignRef(target, nil, suffix)
			if err != nil {
				return nil, err
			}
			slog.Debug("checking for cosign image", slog.String("ref", csRef.Name()))
			csImg, err := d.ofi.Image(match.Name(csRef.Name()))
			if err == nil {
				slog.Debug("found cosign image", slog.String("ref", csRef.Name()))
				csImgs = append(csImgs, ReferencedImage{Ref: csRef, Img: csImg})
				continue
			}
			if errors.Is(err, sif.ErrNoMatch) {
				continue
			}
			return nil, err
		}
	}
	return csImgs, nil
}

// SignedImage returns an image Descriptor as a cosign oci.SignedImage, allowing
// access to signatures and attestations stored alongside the image in the SIF.
func (d *sifDescriptor) SignedImage(ctx context.Context) (cosignoci.SignedImage, error) {
	img, err := d.Image()
	if err != nil {
		return nil, err
	}

	cosignImages, err := d.CosignImages(ctx, false)
	if err != nil {
		return nil, err
	}

	return &sifSignedimage{
		Image:        img,
		cosignImages: cosignImages,
	}, nil
}

type sifSigs struct {
	v1.Image
}

var _ cosignoci.Signatures = (*sifSigs)(nil)

func (s *sifSigs) Get() ([]cosignoci.Signature, error) {
	m, err := s.Manifest()
	if err != nil {
		return nil, err
	}
	signatures := make([]cosignoci.Signature, 0, len(m.Layers))
	for _, desc := range m.Layers {
		layer, err := s.Image.LayerByDigest(desc.Digest)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, cosignsignature.New(layer, desc))
	}
	return signatures, nil
}

type sifSignedimage struct {
	v1.Image
	cosignImages []ReferencedImage
}

var _ cosignoci.SignedImage = (*sifSignedimage)(nil)

func (i *sifSignedimage) Signatures() (cosignoci.Signatures, error) {
	h, err := i.Digest()
	if err != nil {
		return nil, err
	}
	return i.signatures(h, cosignremote.SignatureTagSuffix)
}

func (i *sifSignedimage) Attestations() (cosignoci.Signatures, error) {
	h, err := i.Digest()
	if err != nil {
		return nil, err
	}
	return i.signatures(h, cosignremote.AttestationTagSuffix)
}

func (i *sifSignedimage) signatures(digest v1.Hash, suffix string) (cosignoci.Signatures, error) {
	ref, err := CosignRef(digest, nil, suffix)
	if err != nil {
		return nil, err
	}
	for _, csi := range i.cosignImages {
		if csi.Ref == ref {
			return &sifSigs{Image: csi.Img}, nil
		}
	}
	return cosignempty.Signatures(), nil
}

var errUnsupportedAttachment = errors.New("cosign attachments are not supported")

func (i *sifSignedimage) Attachment(_ string) (cosignoci.File, error) {
	return nil, errUnsupportedAttachment
}

// SignedImageIndex returns an image index Descriptor as a cosign
// oci.SignedImageIndex, allowing access to signatures and attestations stored
// alongside the image in the SIF.
func (d *sifDescriptor) SignedImageIndex(ctx context.Context) (cosignoci.SignedImageIndex, error) {
	if !d.MediaType().IsIndex() {
		return nil, ErrUnsupportedMediaType
	}
	idx, err := d.ImageIndex()
	if err != nil {
		return nil, err
	}

	cosignImages, err := d.CosignImages(ctx, false)
	if err != nil {
		return nil, err
	}

	return &sifSignedImageIndex{
		v1Index:      idx,
		ofi:          d.ofi,
		cosignImages: cosignImages,
	}, nil
}

type v1Index v1.ImageIndex

type sifSignedImageIndex struct {
	v1Index
	ofi          *sif.OCIFileImage
	cosignImages []ReferencedImage
}

var _ cosignoci.SignedImageIndex = (*sifSignedImageIndex)(nil)

func (i *sifSignedImageIndex) Signatures() (cosignoci.Signatures, error) {
	h, err := i.Digest()
	if err != nil {
		return nil, err
	}
	return i.signatures(h, cosignremote.SignatureTagSuffix)
}

func (i *sifSignedImageIndex) Attestations() (cosignoci.Signatures, error) {
	h, err := i.Digest()
	if err != nil {
		return nil, err
	}
	return i.signatures(h, cosignremote.AttestationTagSuffix)
}

func (i *sifSignedImageIndex) SignedImage(h v1.Hash) (cosignoci.SignedImage, error) {
	img, err := i.ofi.Image(match.Digests(h))
	if err != nil {
		return nil, err
	}
	d, err := partial.Descriptor(img)
	if err != nil {
		return nil, err
	}
	mf, err := img.RawManifest()
	if err != nil {
		return nil, err
	}
	parent, err := i.Digest()
	if err != nil {
		return nil, err
	}
	sd := &sifDescriptor{
		descriptor: *d,
		Manifest:   mf,
		ofi:        i.ofi,
		parent:     &parent, // must  be able to traverse index.json -> parent index -> image in OCIDescriptor.Image()
	}
	return sd.SignedImage(context.Background())
}

func (i *sifSignedImageIndex) SignedImageIndex(h v1.Hash) (cosignoci.SignedImageIndex, error) {
	img, err := i.ofi.Index(match.Digests(h))
	if err != nil {
		return nil, err
	}
	d, err := partial.Descriptor(img)
	if err != nil {
		return nil, err
	}
	mf, err := img.RawManifest()
	if err != nil {
		return nil, err
	}
	parent, err := i.Digest()
	if err != nil {
		return nil, err
	}
	sd := &sifDescriptor{
		descriptor: *d,
		Manifest:   mf,
		ofi:        i.ofi,
		parent:     &parent, // must  be able to traverse index.json -> parent index -> image in OCIDescriptor.Image()
	}
	return sd.SignedImageIndex(context.Background())
}

func (i *sifSignedImageIndex) signatures(digest v1.Hash, suffix string) (cosignoci.Signatures, error) {
	ref, err := CosignRef(digest, nil, suffix)
	if err != nil {
		return nil, err
	}
	for _, csi := range i.cosignImages {
		if csi.Ref == ref {
			return &sifSigs{Image: csi.Img}, nil
		}
	}
	return cosignempty.Signatures(), nil
}

func (i *sifSignedImageIndex) Attachment(_ string) (cosignoci.File, error) {
	return nil, errUnsupportedAttachment
}
