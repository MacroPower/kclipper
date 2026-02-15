package helm

import (
	"io"
	"log/slog"
)

type inlineCloser struct {
	closeFn func() error
}

func (c *inlineCloser) Close() error {
	return c.closeFn()
}

func newInlineCloser(closeFn func() error) *inlineCloser {
	return &inlineCloser{closeFn: closeFn}
}

type nopCloser struct{}

func (nopCloser) Close() error {
	return nil
}

// NewNopCloser returns a no-op [io.Closer].
func NewNopCloser() io.Closer {
	return &nopCloser{}
}

// tryClose is a convenience function to tryClose a object that has a Close()
// method, logging any errors.
func tryClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		slog.Warn("close",
			slog.Any("closer", c),
			slog.Any("err", err),
		)
	}
}
