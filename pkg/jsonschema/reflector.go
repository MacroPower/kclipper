package jsonschema

import (
	"fmt"
	"io"
	"reflect"

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

func (r *Reflector) Reflect(t reflect.Type) *Reflected {
	return &Reflected{Schema: r.Reflector.ReflectFromType(t)}
}

type Reflected struct {
	Schema *invopopjsonschema.Schema
}

func (r *Reflected) GenerateKCL(w io.Writer) error {
	jsBytes, err := r.Schema.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal json schema: %w", err)
	}

	if err := kclutil.Gen.GenKcl(w, "chart", jsBytes, &gen.GenKclOptions{
		Mode:          gen.ModeJsonSchema,
		CastingOption: gen.OriginalName,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
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
