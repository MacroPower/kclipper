package jsonschema

import (
	invopopjsonschema "github.com/invopop/jsonschema"
)

type Reflector struct {
	r *invopopjsonschema.Reflector
}

func NewReflector() *Reflector {
	return &Reflector{
		r: &invopopjsonschema.Reflector{
			DoNotReference: true,
			ExpandedStruct: true,
		},
	}
}

func (r *Reflector) Reflect(v interface{}) *invopopjsonschema.Schema {
	return r.r.Reflect(v)
}
