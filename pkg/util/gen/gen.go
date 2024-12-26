package gen

import (
	"fmt"
	"io"
	"sync"

	"kcl-lang.io/kcl-go/pkg/tools/gen"
)

// Safe is a concurrency-safe KCL generator.
var Safe = &safe{}

type safe struct {
	mu sync.Mutex
}

func (g *safe) GenKcl(w io.Writer, filename string, src interface{}, opts *gen.GenKclOptions) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := gen.GenKcl(w, filename, src, opts); err != nil {
		return fmt.Errorf("failed to generate kcl: %w", err)
	}

	return nil
}
