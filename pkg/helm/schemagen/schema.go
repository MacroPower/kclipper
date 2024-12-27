// Copyright (c) 2023 dadav. MIT License

package schemagen

type Generator string

const (
	AutoGenerator      Generator = "AUTO"
	ValuesGenerator    Generator = "VALUE-INFERENCE"
	URLGenerator       Generator = "URL"
	PathGenerator      Generator = "PATH"
	LocalPathGenerator Generator = "LOCAL-PATH"
	NoGenerator        Generator = "NONE"
)

var (
	Generators = []Generator{
		AutoGenerator,
		ValuesGenerator,
		URLGenerator,
		PathGenerator,
		LocalPathGenerator,
		NoGenerator,
	}
	GeneratorEnum = []interface{}{
		AutoGenerator,
		ValuesGenerator,
		URLGenerator,
		PathGenerator,
		LocalPathGenerator,
		NoGenerator,
	}
)
