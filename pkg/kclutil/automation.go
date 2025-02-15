package kclutil

import (
	"fmt"
	"sort"
	"strings"
)

type MapValue struct {
	s *string
	b *bool
}

func (s MapValue) IsString() bool {
	return s.s != nil
}

func (s MapValue) IsBool() bool {
	return s.b != nil
}

func (s MapValue) GetValue() string {
	if s.IsString() {
		if *s.s != "" {
			return fmt.Sprintf(`"%s"`, *s.s)
		}

		return ""
	}

	if s.IsBool() {
		if *s.b {
			return "True"
		}

		return ""
	}

	return ""
}

func NewString(s string) MapValue {
	return MapValue{s: &s}
}

func NewBool(b bool) MapValue {
	return MapValue{b: &b}
}

type Automation map[string]MapValue

// GetSpecs returns a sorted list of specs which can be passed to [kcl-lang.io/kcl-go.OverrideFile].
func (a Automation) GetSpecs(specPath string) ([]string, error) {
	specs := sort.StringSlice{}

	for k, v := range a {
		if k == "" {
			return nil, fmt.Errorf("invalid key in KCL automation: %#v", a)
		}

		val := v.GetValue()
		if val == "" {
			continue
		}

		specs = append(specs, fmt.Sprintf(`%s=%s`, SpecPathJoin(specPath, k), val))
	}

	specs.Sort()

	return specs, nil
}

func SpecPathJoin(path ...string) string {
	pathParts := []string{}
	for _, p := range path {
		pathParts = append(pathParts, strings.FieldsFunc(p, func(c rune) bool {
			return c == '.'
		})...)
	}

	return strings.Join(pathParts, ".")
}
