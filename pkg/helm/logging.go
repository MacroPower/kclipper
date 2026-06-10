package helm

import (
	"context"
	"fmt"
	"log/slog"
)

// newDebugHandler creates a new [debugHandler] delegating to the default
// [slog.Handler].
func newDebugHandler() debugHandler {
	return debugHandler{Handler: slog.Default().Handler()}
}

// debugHandler demotes all records to [slog.LevelDebug] before delegating to
// the wrapped [slog.Handler]. It keeps Helm SDK logs, which are emitted at
// various levels, on the debug level only. Helm only routes logs through its
// action configurations in some packages; others write directly to the
// default logger and are not demoted. Create instances with
// [newDebugHandler].
type debugHandler struct {
	slog.Handler
}

func (h debugHandler) Enabled(ctx context.Context, _ slog.Level) bool {
	return h.Handler.Enabled(ctx, slog.LevelDebug)
}

//nolint:gocritic // hugeParam: signature required by [slog.Handler].
func (h debugHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Level = slog.LevelDebug

	err := h.Handler.Handle(ctx, r)
	if err != nil {
		return fmt.Errorf("handle record: %w", err)
	}

	return nil
}

func (h debugHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return debugHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h debugHandler) WithGroup(name string) slog.Handler {
	return debugHandler{Handler: h.Handler.WithGroup(name)}
}
