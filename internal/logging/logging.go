// Package logging provides structured debug logging for muxwarp.
//
// Call Init with a file path to enable JSON logging; call it with an empty
// string (or not at all) for a silent no-op logger. Every other package
// uses Log() to obtain the shared logger — no function signatures change.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewJSONHandler(io.Discard, nil))

// Init configures the package-level logger. An empty path installs a
// silent (discard) logger. A non-empty path opens the file in append
// mode and writes JSON log lines to it. The returned cleanup function
// closes the file; it is nil when path is empty.
func Init(path string) (cleanup func(), err error) {
	if path == "" {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
		return nil, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return func() { f.Close() }, nil
}

// Log returns the package-level logger.
func Log() *slog.Logger { return logger }
