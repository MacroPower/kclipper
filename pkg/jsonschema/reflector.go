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

func (r *Reflector) Reflect(t reflect.Type) *invopopjsonschema.Schema {
	return r.Reflector.ReflectFromType(t)
}

func ReflectedSchemaToKCL(r *invopopjsonschema.Schema, w io.Writer) error {
	jsBytes, err := r.MarshalJSON()
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
