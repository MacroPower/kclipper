package helm

import (
	"io"
	"log/slog"
)

type InlineCloser struct {
	closeFn func() error
}

func (c *InlineCloser) Close() error {
	return c.closeFn()
}

func newInlineCloser(closeFn func() error) *InlineCloser {
	return &InlineCloser{closeFn: closeFn}
}

// tryClose is a convenience function to tryClose a object that has a Close()
// method, logging any errors.
func tryClose(c io.Closer) {
	if err := c.Close(); err != nil {
		slog.Warn("failed to close", "closer", c, "err", err)
	}
}
