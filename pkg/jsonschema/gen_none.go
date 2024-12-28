package jsonschema

import "errors"

var DefaultNoGenerator = NewNoGenerator()

var _ Generator = DefaultNoGenerator

type NoGenerator struct{}

func NewNoGenerator() *NoGenerator {
	return &NoGenerator{}
}

func (g *NoGenerator) FromData(_ []byte) ([]byte, error) {
	return nil, errors.New("no generator selected")
}

func (g *NoGenerator) FromPaths(_ ...string) ([]byte, error) {
	return nil, errors.New("no generator selected")
}
