// Copyright 2024-2025 Sylabs Inc. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package sourcesink

import (
	"log/slog"
)

// SourceSink implements methods to read / write images and indexes from / to a
// specific type of storage, and location.
type SourceSink interface {
	Source
	Sink
}

// options holds options that should apply across multiple Get / Write
// operations against a source or sink.
type options struct {
	instrumentationLogger *slog.Logger
}

// Option sets an option that applies across multiple Get / Write operations against
// a source or sink.
type Option func(*options) error

// OptWithInstrumentationLogs adds instrumentation of operations against the underlying image.
func OptWithInstrumentationLogs(l *slog.Logger) Option {
	return func(o *options) error {
		o.instrumentationLogger = l
		return nil
	}
}
