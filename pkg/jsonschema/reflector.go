package jsonschema

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"

	invopopjsonschema "github.com/invopop/jsonschema"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

type Reflector struct {
	Reflector *invopopjsonschema.Reflector
}

func NewReflector() *Reflector {
	return &Reflector{
		Reflector: &invopopjsonschema.Reflector{
			DoNotReference: true,
			ExpandedStruct: true,
		},
	}
}

func (r *Reflector) AddGoComments(pkg, path string) error {
	err := r.Reflector.AddGoComments(pkg, path, invopopjsonschema.WithFullComment())
	if err != nil {
		return fmt.Errorf("failed to add go comments from '%s': %w", pkg, err)
	}
	return nil
}

func (r *Reflector) Reflect(t reflect.Type, opts ...PropertyOpt) *Reflected {
	rs := r.Reflector.ReflectFromType(t)
	for _, opt := range opts {
		opt(rs)
	}
	return &Reflected{Schema: rs}
}

type replacement struct {
	re   *regexp.Regexp
	repl string
}

type Reflected struct {
	Schema *invopopjsonschema.Schema

	replacements []replacement
}

func (r *Reflected) GenerateKCL(w io.Writer, opts ...GenOpt) error {
	for _, opt := range opts {
		opt(r)
	}

	jsBytes, err := r.Schema.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal json schema: %w", err)
	}

	b := &bytes.Buffer{}
	if err := kclutil.Gen.GenKcl(b, "chart", jsBytes, &kclutil.GenKclOptions{
		Mode:          gen.ModeJsonSchema,
		CastingOption: gen.OriginalName,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
	}

	for _, v := range r.replacements {
		kclData := b.Bytes()
		b.Reset()
		b.Write(v.re.ReplaceAll(kclData, []byte(v.repl)))
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("failed to write KCL schema: %w", err)
	}

	return nil
}

func (r *Reflected) SetProperty(key string, opts ...PropertyOpt) {
	if cv, ok := r.Schema.Properties.Get(key); ok {
		for _, opt := range opts {
			opt(cv)
		}
	}
}

func (r *Reflected) SetOrRemoveProperty(key string, setProperty bool, opts ...PropertyOpt) {
	if cv, ok := r.Schema.Properties.Get(key); ok {
		if setProperty {
			for _, opt := range opts {
				opt(cv)
			}
		} else {
			r.Schema.Properties.Delete(key)
		}
	}
}

func (r *Reflected) RemoveProperty(key string) {
	if _, ok := r.Schema.Properties.Get(key); ok {
		r.Schema.Properties.Delete(key)
	}
}

type GenOpt func(*Reflected)

func Replace(re *regexp.Regexp, repl string) GenOpt {
	return func(r *Reflected) {
		r.replacements = append(r.replacements, replacement{re: re, repl: repl})
	}
}

type PropertyOpt func(*invopopjsonschema.Schema)

func WithEnum(enum []interface{}) PropertyOpt {
	return func(s *invopopjsonschema.Schema) {
		s.Enum = enum
	}
}

func WithDefault(defaultValue interface{}) PropertyOpt {
	return func(s *invopopjsonschema.Schema) {
		s.Default = defaultValue
	}
}

func WithType(t string) PropertyOpt {
	return func(s *invopopjsonschema.Schema) {
		s.Type = t
	}
}

func WithNoItems() PropertyOpt {
	return func(s *invopopjsonschema.Schema) {
		s.Items = nil
	}
}

func WithAllowAdditionalProperties() PropertyOpt {
	return func(s *invopopjsonschema.Schema) {
		s.AdditionalProperties = invopopjsonschema.TrueSchema
	}
}
