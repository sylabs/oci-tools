// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package instrumented

import (
	"io"
	"log/slog"
	"time"
)

type wrappedReadCloser struct {
	inner     io.ReadCloser
	log       *slog.Logger
	createdAt time.Time
	count     int
}

func readCloser(rc io.ReadCloser, log *slog.Logger) io.ReadCloser {
	return &wrappedReadCloser{
		inner:     rc,
		log:       log,
		createdAt: time.Now(),
	}
}

func (rc *wrappedReadCloser) Read(p []byte) (int, error) {
	n, err := rc.inner.Read(p)
	rc.count += n
	return n, err
}

func (rc *wrappedReadCloser) Close() error {
	rc.log.Info("Close()",
		slog.Duration("dur", time.Since(rc.createdAt)),
		slog.Int("count", rc.count),
	)

	return rc.inner.Close()
}
