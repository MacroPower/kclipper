// Copyright (c) 2023 dadav. MIT License

package schemagen

type Generator string

const (
	AutoGenerator   Generator = "AUTO"
	ValuesGenerator Generator = "VALUE-INFERENCE"
	URLGenerator    Generator = "URL"
	NoGenerator     Generator = "NONE"
)

var (
	Generators    = []Generator{AutoGenerator, ValuesGenerator, URLGenerator, NoGenerator}
	GeneratorEnum = []interface{}{AutoGenerator, ValuesGenerator, URLGenerator, NoGenerator}
)
