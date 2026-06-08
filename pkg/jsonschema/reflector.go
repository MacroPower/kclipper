package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"

	gojsonschema "github.com/google/jsonschema-go/jsonschema"
	xjsonschema "go.jacobcolvin.com/x/jsonschema"

	"github.com/macropower/kclipper/pkg/kclgen"
)

// reflectConfig holds the options for a [Reflect] call.
type reflectConfig struct {
	comments bool
}

// ReflectOpt configures [Reflect].
//
// Available options:
//   - [WithGoComments]
type ReflectOpt func(*reflectConfig)

// WithGoComments is a [ReflectOpt] that enables extraction of Go doc comments
// as schema descriptions. The source files of the reflected types must be
// resolvable at generation time.
func WithGoComments() ReflectOpt {
	return func(c *reflectConfig) {
		c.comments = true
	}
}

// Reflect generates a schema for t and returns it wrapped in a [Reflected].
//
// Definitions are inlined rather than referenced via $ref, and nil-able Go
// types are not made nullable, so the result is a single self-contained,
// non-nullable schema suitable for KCL schema generation (a multi-type union
// with null would otherwise collapse to any).
func Reflect(t reflect.Type, opts ...ReflectOpt) (*Reflected, error) {
	var cfg reflectConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	genOpts := []xjsonschema.Option{
		// Inline every type instead of emitting a $defs/$ref graph; KCL schema
		// generation consumes a single flattened schema.
		xjsonschema.WithDefinitions(false),
		// Keep slices, maps, and pointers non-nullable; KCL schema generation
		// renders a type union with null as any, losing the element type.
		xjsonschema.WithNullable(false),
	}
	if cfg.comments {
		genOpts = append(genOpts, xjsonschema.WithComments(true))
	}

	s, err := xjsonschema.Generate(t, genOpts...)
	if err != nil {
		return nil, fmt.Errorf("reflect %s: %w", t, err)
	}

	return &Reflected{Schema: s, name: typeName(t)}, nil
}

// typeName returns the schema name for t, dereferencing pointers. KCL schema
// generation names the root schema after this value, since the inlined schema
// carries no $id for it to use instead.
func typeName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

type replacement struct {
	re   *regexp.Regexp
	repl string
}

// Reflected is a JSON Schema produced by [Reflect], ready to be converted to a
// KCL schema. Create instances with [Reflect].
type Reflected struct {
	// Schema is the underlying JSON Schema. It can be adjusted directly or via
	// [Reflected.SetProperty] and related methods before KCL generation.
	Schema *gojsonschema.Schema

	name         string
	replacements []replacement
}

// GenerateKCL converts the schema to a KCL schema and writes it to w.
func (r *Reflected) GenerateKCL(w io.Writer, opts ...GenOpt) error {
	for _, opt := range opts {
		opt(r)
	}

	jsBytes, err := r.Schema.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal json schema: %w", err)
	}

	// KCL schema generation names the root schema after the file name when the
	// schema has no $id; pass the reflected type name so the schema keeps it.
	filename := r.name
	if filename == "" {
		filename = "chart"
	}

	b := &bytes.Buffer{}
	err = kclgen.Gen.GenKcl(b, filename, jsBytes, &kclgen.GenKclOptions{
		Mode:          kclgen.ModeJSONSchema,
		CastingOption: kclgen.OriginalName,
	})
	if err != nil {
		return fmt.Errorf("generate kcl schema: %w", err)
	}

	for _, v := range r.replacements {
		kclData := b.Bytes()
		b.Reset()
		b.Write(v.re.ReplaceAll(kclData, []byte(v.repl)))
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("write KCL schema: %w", err)
	}

	return nil
}

// SetProperty applies opts to the property named key, if it exists.
func (r *Reflected) SetProperty(key string, opts ...PropertyOpt) {
	if cv, ok := r.Schema.Properties[key]; ok {
		for _, opt := range opts {
			opt(cv)
		}
	}
}

// SetOrRemoveProperty applies opts to the property named key when setProperty
// is true, and otherwise removes the property.
func (r *Reflected) SetOrRemoveProperty(key string, setProperty bool, opts ...PropertyOpt) {
	cv, ok := r.Schema.Properties[key]
	if !ok {
		return
	}

	if setProperty {
		for _, opt := range opts {
			opt(cv)
		}

		return
	}

	delete(r.Schema.Properties, key)
}

// RemoveProperty removes the property named key.
func (r *Reflected) RemoveProperty(key string) {
	delete(r.Schema.Properties, key)
}

// GenOpt configures KCL generation from a [Reflected].
//
// Available options:
//   - [Replace]
type GenOpt func(*Reflected)

// Replace is a [GenOpt] that registers a regular-expression replacement applied
// to the generated KCL schema text.
func Replace(re *regexp.Regexp, repl string) GenOpt {
	return func(r *Reflected) {
		r.replacements = append(r.replacements, replacement{re: re, repl: repl})
	}
}

// PropertyOpt modifies a property [gojsonschema.Schema].
//
// Available options:
//   - [WithEnum]
//   - [WithItemsEnum]
//   - [WithDefault]
//   - [WithType]
//   - [WithNoContent]
//   - [WithAllowAdditionalProperties]
type PropertyOpt func(*gojsonschema.Schema)

// WithEnum is a [PropertyOpt] that sets an enum on a property schema.
func WithEnum(enum []any) PropertyOpt {
	return func(s *gojsonschema.Schema) {
		s.Enum = enum
	}
}

// WithItemsEnum is a [PropertyOpt] that sets an enum on an array property's
// items schema.
func WithItemsEnum(enum []any) PropertyOpt {
	return func(s *gojsonschema.Schema) {
		if s.Items != nil {
			s.Items.Enum = enum
		}
	}
}

// WithDefault is a [PropertyOpt] that sets the default on a property schema. A
// nil value leaves the default unset, matching JSON omitempty semantics.
func WithDefault(defaultValue any) PropertyOpt {
	return func(s *gojsonschema.Schema) {
		if defaultValue == nil {
			return
		}

		data, err := json.Marshal(defaultValue)
		if err != nil {
			return
		}

		s.Default = data
	}
}

// WithType is a [PropertyOpt] that sets a single type on a property schema,
// clearing any multi-type union.
func WithType(t string) PropertyOpt {
	return func(s *gojsonschema.Schema) {
		s.Type = t
		s.Types = nil
	}
}

// WithNoContent is a [PropertyOpt] that removes the items and properties from a
// property schema.
func WithNoContent() PropertyOpt {
	return func(s *gojsonschema.Schema) {
		s.Items = nil
		s.Properties = nil
	}
}

// WithAllowAdditionalProperties is a [PropertyOpt] that allows additional
// properties on a property schema.
func WithAllowAdditionalProperties() PropertyOpt {
	return func(s *gojsonschema.Schema) {
		s.AdditionalProperties = &gojsonschema.Schema{}
	}
}
