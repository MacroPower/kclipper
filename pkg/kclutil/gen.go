package kclutil

import (
	"bytes"
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

type GenKclOptions struct {
	Mode                  kclgen.Mode
	CastingOption         kclgen.CastingOption
	UseIntegersForNumbers bool
	RemoveDefaults        bool
}

func (g *gen) GenKcl(w io.Writer, filename string, src interface{}, opts *GenKclOptions) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	kclSchemaBuf := &bytes.Buffer{}
	kgo := &kclgen.GenKclOptions{
		Mode:                  opts.Mode,
		CastingOption:         opts.CastingOption,
		UseIntegersForNumbers: opts.UseIntegersForNumbers,
	}

	if err := kclgen.GenKcl(kclSchemaBuf, filename, src, kgo); err != nil {
		return fmt.Errorf("failed to generate kcl: %w", err)
	}

	kclSchema := FixKCLSchema(kclSchemaBuf.String(), opts.RemoveDefaults)
	if _, err := w.Write([]byte(kclSchema)); err != nil {
		return fmt.Errorf("failed to write KCL schema: %w", err)
	}

	return nil
}
