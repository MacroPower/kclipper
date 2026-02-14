package kclautomation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/macropower/kclipper/pkg/kclerrors"
)

// MapValue represents a value that can be either a string or a boolean.
type MapValue struct {
	s *string
	b *bool
}

// IsString returns true if the value is a string.
func (s MapValue) IsString() bool {
	return s.s != nil
}

// IsBool returns true if the value is a boolean.
func (s MapValue) IsBool() bool {
	return s.b != nil
}

// GetValue returns the string representation of the value.
func (s MapValue) GetValue() string {
	if s.IsString() {
		if *s.s != "" {
			return fmt.Sprintf("%q", *s.s)
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

// NewString creates a new MapValue from a string.
func NewString(s string) MapValue {
	return MapValue{s: &s}
}

// NewBool creates a new MapValue from a boolean.
func NewBool(b bool) MapValue {
	return MapValue{b: &b}
}

// Automation represents a collection of keys and their associated values for automation.
type Automation map[string]MapValue

// GetSpecs returns a sorted list of specs which can be passed to [kcl-lang.io/kcl-go.OverrideFile].
func (a Automation) GetSpecs(specPath string) ([]string, error) {
	specs := sort.StringSlice{}

	for k, v := range a {
		if k == "" {
			return nil, fmt.Errorf("%w: empty key in KCL automation", kclerrors.ErrInvalidFormat)
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

// SpecPathJoin joins path components with dots, splitting any components that already contain dots.
func SpecPathJoin(path ...string) string {
	pathParts := make([]string, 0, len(path))
	for _, p := range path {
		pathParts = append(pathParts, strings.FieldsFunc(p, func(c rune) bool {
			return c == '.'
		})...)
	}

	return strings.Join(pathParts, ".")
}
