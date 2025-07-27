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

type NopCloser struct{}

func (NopCloser) Close() error {
	return nil
}

func NewNopCloser() io.Closer {
	return &NopCloser{}
}

// tryClose is a convenience function to tryClose a object that has a Close()
// method, logging any errors.
func tryClose(c io.Closer) {
	err := c.Close()
	if err != nil {
		slog.Warn("failed to close",
			slog.Any("closer", c),
			slog.Any("err", err),
		)
	}
}
