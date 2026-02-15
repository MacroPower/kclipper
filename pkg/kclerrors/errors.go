package kclerrors

import (
	"errors"
	"fmt"
)

var (
	// ErrWrite indicates an error occurred while writing.
	ErrWrite = errors.New("write")

	// ErrWriteFile indicates an error occurred while writing a file.
	ErrWriteFile = fmt.Errorf("file: %w", ErrWrite)

	// ErrGenerateKCL indicates an error occurred during KCL generation.
	ErrGenerateKCL = errors.New("generate KCL")

	// ErrSchemaNotFound indicates a schema wasn't found in the available schemas.
	ErrSchemaNotFound = errors.New("schema not found")

	// ErrInvalidFormat indicates an unexpected or invalid format was encountered.
	ErrInvalidFormat = errors.New("invalid format")

	// ErrFileNotFound indicates a file wasn't found in the specified path.
	ErrFileNotFound = errors.New("file not found")

	// ErrOverrideFile indicates an error occurred while overriding a file.
	ErrOverrideFile = errors.New("override file")

	// ErrParseArgs indicates an error occurred while parsing arguments.
	ErrParseArgs = errors.New("parse arguments")

	// ErrInvalidArguments indicates invalid arguments were provided.
	ErrInvalidArguments = errors.New("invalid arguments")

	// ErrJSONMarshal indicates an error occurred while marshaling JSON.
	ErrJSONMarshal = errors.New("marshal JSON")

	// ErrYAMLMarshal indicates an error occurred while marshaling YAML.
	ErrYAMLMarshal = errors.New("marshal YAML")
)
