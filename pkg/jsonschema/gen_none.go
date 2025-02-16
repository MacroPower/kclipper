package jsonschema

var (
	DefaultNoGenerator = NewNoGenerator()

	_ FileGenerator = DefaultNoGenerator
)

const EmptySchema string = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "additionalProperties": true,
  "required": [],
  "type": "object"
}`

// NoGenerator always returns an empty JSON Schema.
type NoGenerator struct{}

// NewNoGenerator creates a new [NoGenerator].
func NewNoGenerator() *NoGenerator {
	return &NoGenerator{}
}

// FromData returns an empty JSON Schema, regardless of the input data.
func (g *NoGenerator) FromData(_ []byte) ([]byte, error) {
	return []byte(EmptySchema), nil
}

// FromPaths returns an empty JSON Schema, regardless of the input paths.
func (g *NoGenerator) FromPaths(_ ...string) ([]byte, error) {
	return []byte(EmptySchema), nil
}
