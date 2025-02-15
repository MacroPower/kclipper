package kclutil

import (
	"regexp"
)

var (
	schemaDefaultMultilineRegexp = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)\s+=\s*r"""([\s\S]*?)"""`)
	schemaDefaultRegexp          = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=.+)`)
)

func FixKCLSchema(kclSchema string, removeDefaults bool) string {
	if removeDefaults {
		kclSchema = schemaDefaultMultilineRegexp.ReplaceAllString(kclSchema, "$1")
		kclSchema = schemaDefaultRegexp.ReplaceAllString(kclSchema, "$1")
	}

	return kclSchema
}
