package kclutil

import (
	"fmt"
	"io"
	"sync"

	kclgen "kcl-lang.io/kcl-go/pkg/tools/gen"
)

// Gen is a concurrency-safe KCL generator.
var Gen = &gen{}

type gen struct {
	mu sync.Mutex
}

func (g *gen) GenKcl(w io.Writer, filename string, src interface{}, opts *kclgen.GenKclOptions) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := kclgen.GenKcl(w, filename, src, opts); err != nil {
		return fmt.Errorf("failed to generate kcl: %w", err)
	}

	return nil
}
