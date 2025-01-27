package jsonschema

var DefaultNoGenerator = NewNoGenerator()

var _ FileGenerator = DefaultNoGenerator

type NoGenerator struct{}

func NewNoGenerator() *NoGenerator {
	return &NoGenerator{}
}

func (g *NoGenerator) FromData(_ []byte) ([]byte, error) {
	return []byte{}, nil
}

func (g *NoGenerator) FromPaths(_ ...string) ([]byte, error) {
	return []byte{}, nil
}
