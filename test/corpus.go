// Copyright 2023 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package test

type Corpus struct {
	dir string
}

// NewCorpus returns a new Corpus. The path specifies the location of the "test" directory.
func NewCorpus(path string) *Corpus {
	return &Corpus{path}
}
