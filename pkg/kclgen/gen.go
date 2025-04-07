package kclgen

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	gentool "kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
)

// Gen is a concurrency-safe KCL generator.
var Gen = &gen{}

type gen struct {
	mu sync.Mutex
}

// GenKclOptions contains options for KCL generation.
type GenKclOptions struct {
	Mode                  Mode
	CastingOption         CastingOption
	UseIntegersForNumbers bool
	RemoveDefaults        bool
}

type (
	// Mode is the mode of KCL schema code generation.
	Mode int

	// CastingOption is the option for casting field names.
	CastingOption int
)

const (
	ModeAuto Mode = iota
	ModeGoStruct
	ModeJSONSchema
	ModeTerraformSchema
	ModeJSON
	ModeYAML
	ModeTOML
	ModeProto
	ModeTextProto

	OriginalName CastingOption = iota
	SnakeCase
	CamelCase
)

// GenKcl generates KCL schema with the provided options.
func (g *gen) GenKcl(w io.Writer, filename string, src any, opts *GenKclOptions) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	kclSchemaBuf := &bytes.Buffer{}
	kgo := &gentool.GenKclOptions{
		Mode:                  gentool.Mode(opts.Mode),
		CastingOption:         gentool.CastingOption(opts.CastingOption),
		UseIntegersForNumbers: opts.UseIntegersForNumbers,
	}

	if err := gentool.GenKcl(kclSchemaBuf, filename, src, kgo); err != nil {
		return fmt.Errorf("%w: %w", kclerrors.ErrGenerateKCL, err)
	}

	kclSchema := FixKCLSchema(kclSchemaBuf.String(), opts.RemoveDefaults)
	if _, err := w.Write([]byte(kclSchema)); err != nil {
		return fmt.Errorf("%w: %w", kclerrors.ErrWrite, err)
	}

	return nil
}
