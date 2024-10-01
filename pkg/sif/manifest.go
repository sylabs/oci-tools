package sif

import (
	"encoding/json"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type editedManifest struct {
	base v1.ImageIndex
	im   *v1.IndexManifest
}

// MediaType of this index's manifest.
func (in *editedManifest) MediaType() (types.MediaType, error) {
	return in.base.MediaType()
}

// Digest returns the sha256 of this index's manifest.
func (in *editedManifest) Digest() (v1.Hash, error) {
	return partial.Digest(in)
}

// Size returns the size of the manifest.
func (in *editedManifest) Size() (int64, error) {
	return partial.Size(in)
}

// IndexManifest returns this image index's manifest object.
func (in *editedManifest) IndexManifest() (*v1.IndexManifest, error) {
	return in.im, nil
}

// RawManifest returns the serialized bytes of IndexManifest().
func (in *editedManifest) RawManifest() ([]byte, error) {
	return json.Marshal(in.im)
}

// Image returns a v1.Image that this ImageIndex references.
func (in *editedManifest) Image(h v1.Hash) (v1.Image, error) {
	return in.base.Image(h)
}

// ImageIndex returns a v1.ImageIndex that this ImageIndex references.
func (in *editedManifest) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	return in.base.ImageIndex(h)
}

type descriptorEditFunc func(desc v1.Descriptor) v1.Descriptor

// editManifestDescriptors edits the index manifest described by ii. Each descriptor that matches m
// is replaced by the descriptor returned by fn.
func editManifestDescriptors(ii v1.ImageIndex, m match.Matcher, fn descriptorEditFunc) (v1.ImageIndex, error) {
	im, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	im = im.DeepCopy()
	for i, d := range im.Manifests {
		if m(d) {
			im.Manifests[i] = fn(d)
		}
	}

	return &editedManifest{
		base: ii,
		im:   im,
	}, nil
}
