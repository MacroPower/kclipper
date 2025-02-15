package kclutil

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var (
	schemaInvalidDefaultRegexp   = regexp.MustCompile(`( default is\s+)r"""([\s\S]*?)"""(.*)`)
	schemaDefaultMultilineRegexp = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)\s+=\s*r"""([\s\S]*?)"""`)
	schemaDefaultRegexp          = regexp.MustCompile(`(\s+\S+:\s+\S+(\s+\|\s+\S+)*)(\s+=.+)`)
)

func FixKCLSchema(kclSchema string, removeDefaults bool) string {
	kclSchema = schemaInvalidDefaultRegexp.ReplaceAllStringFunc(kclSchema, fixMultilineKCLDefaultComments)

	if removeDefaults {
		kclSchema = schemaDefaultMultilineRegexp.ReplaceAllString(kclSchema, "$1")
		kclSchema = schemaDefaultRegexp.ReplaceAllString(kclSchema, "$1")
	}

	return kclSchema
}

func fixMultilineKCLDefaultComments(s string) string {
	submatches := schemaInvalidDefaultRegexp.FindStringSubmatch(s)
	if len(submatches) != 4 {
		panic(fmt.Sprintf("regex had %d submatches in %q; expected 4 submatches", len(submatches), s))
	}

	indent := strings.Repeat(" ", 8)
	fixedContent := bytes.NewBufferString("\n")
	mustWrite(fixedContent, (indent + "default is\n"))
	mustWrite(fixedContent, (indent + "```"))

	for _, line := range strings.Split(submatches[2], "\n") {
		mustWrite(fixedContent, "\n")

		if line == "" {
			continue
		}

		mustWrite(fixedContent, (indent + line))
	}

	mustWrite(fixedContent, "\n")
	mustWrite(fixedContent, (indent + "```"))

	return fixedContent.String()
}

func mustWrite(w *bytes.Buffer, s string) {
	if _, err := w.WriteString(s); err != nil {
		panic(fmt.Errorf("failed to write to buffer: %w", err))
	}
}
