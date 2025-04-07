package kclerrors

import (
	"errors"
	"fmt"
)

var (
	// ErrWrite indicates an error occurred while writing.
	ErrWrite = errors.New("failed to write")

	// ErrWriteFile indicates an error occurred while writing a file.
	ErrWriteFile = fmt.Errorf("file: %w", ErrWrite)

	// ErrOverrideFile indicates an error occurred while overriding a file.
	ErrOverrideFile = errors.New("failed to override file")

	// ErrGenerateKCL indicates an error occurred during KCL generation.
	ErrGenerateKCL = errors.New("failed to generate KCL")

	// ErrParseArgs indicates an error occurred while parsing arguments.
	ErrParseArgs = errors.New("failed to parse arguments")

	// ErrInvalidArguments indicates that the provided arguments are invalid.
	ErrInvalidArguments = errors.New("invalid arguments")

	// ErrSchemaNotFound indicates a schema wasn't found in the available schemas.
	ErrSchemaNotFound = errors.New("schema not found")

	// ErrInvalidFormat indicates an unexpected or invalid format was encountered.
	ErrInvalidFormat = errors.New("invalid format")

	// ErrJSONMarshal indicates an error occurred during JSON marshaling.
	ErrJSONMarshal = errors.New("failed to marshal JSON")

	// ErrYAMLMarshal indicates an error occurred during YAML marshaling.
	ErrYAMLMarshal = errors.New("failed to marshal YAML")

	// ErrFileNotFound indicates a file wasn't found in the specified path.
	ErrFileNotFound = errors.New("file not found")

	// ErrResolvedOutsideRepo indicates a file was resolved to outside the repository root.
	ErrResolvedOutsideRepo = errors.New("file resolved to outside repository root")
)
