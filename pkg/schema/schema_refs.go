package schema

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"go.jacobcolvin.com/x/jsonschema"
)

// refRootName is the placeholder base name [inlineSchemaRefs] gives the root
// document. A single path segment with no separators lets relative file refs
// absolutize to bare file names that [yamlFileResolver] reads from the
// reference base directory; the name itself never appears in the output.
const refRootName = "root.json"

// inlineSchemaRefs expands every $ref in schema into a copy of its target,
// returning a self-contained schema. Relative file refs resolve against
// refBasePath (the current directory when empty), and fragment refs
// (#/pointer, #anchor) resolve within their document.
//
// It targets Draft-07 semantics so a $ref replaces its node outright, dropping
// sibling keywords, matching how kclipper flattens schemas for KCL generation.
// Failures follow [refResolveFallback]; a reference cycle surfaces as an error
// wrapping [jsonschema.ErrRefCycle].
func inlineSchemaRefs(ctx context.Context, schema *jsonschema.Schema, refBasePath string) (*jsonschema.Schema, error) {
	baseDir := refBasePath
	if baseDir == "" {
		baseDir = "."
	}

	inlined, err := jsonschema.Inline(ctx, schema,
		jsonschema.WithDraft(jsonschema.Draft7),
		jsonschema.WithRefResolver(yamlFileResolver{fsys: os.DirFS(baseDir)}),
		jsonschema.WithBaseURI(refRootName),
		// Resolve refs by on-disk location, treating any published remote $id
		// as inert. Vendored schemas (e.g. Helm library charts) routinely
		// declare a remote $id while shipping the files their relative refs
		// name alongside them on disk.
		jsonschema.WithRetrievalBase(true),
		jsonschema.WithRefFallback(jsonschema.RefFallbackFunc(refResolveFallback)),
	)
	if err != nil {
		if errors.Is(err, jsonschema.ErrRefCycle) {
			return nil, fmt.Errorf("circular reference: %w", err)
		}

		return nil, fmt.Errorf("inline schema refs: %w", err)
	}

	return inlined, nil
}

// yamlFileResolver resolves file refs from an [io/fs.FS], accepting YAML or
// JSON schema documents. It mirrors [jsonschema.FileResolver] but reads
// through [unmarshalSchema], so a ref target written as YAML resolves the same
// as one written as JSON. See [jsonschema.RefResolver] for the interface.
type yamlFileResolver struct {
	fsys fs.FS
}

// ResolveRef reads and decodes the schema document named by uri, confining
// resolution to the fs root the way [jsonschema.FileResolver] does: the
// "file://" scheme and a leading "/" are stripped, and the remainder is the
// [io/fs] path. Reads are local and not cancellable, so the context is unused.
func (r yamlFileResolver) ResolveRef(_ context.Context, uri string) (*jsonschema.Schema, error) {
	name := strings.TrimPrefix(uri, "file://")
	name = strings.TrimPrefix(name, "/")

	data, err := fs.ReadFile(r.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read schema document %q: %w", name, err)
	}

	return unmarshalSchema(data)
}

// refResolveFallback is the per-reference failure policy for [inlineSchemaRefs],
// tolerating the partial, hand-vendored schemas kclipper consumes (e.g.
// Kubernetes API subsets and Helm library-chart values schemas):
//
//   - A reference cycle is always fatal, surfacing the mutually-referencing
//     documents instead of silently truncating the schema.
//   - An unresolved fragment ref (#/pointer, #anchor) is dropped, leaving the
//     rest of the node intact. Partial imports routinely reference definitions
//     that were not carried along, and such a ref constrains nothing once
//     dropped.
//   - An unresolved ref anywhere inside an additionalProperties subtree is
//     dropped, widening that position to the permissive empty schema. Values
//     schemas commonly point additionalProperties at a remote document
//     (e.g. a library chart's per-key schema served over HTTP) that is not
//     fetched during generation.
//   - Every other failure — an unresolvable external document at a fixed
//     position, a construct with no static expansion — is fatal.
func refResolveFallback(_ context.Context, f jsonschema.RefFailure) jsonschema.RefAction {
	if errors.Is(f.Err, jsonschema.ErrRefCycle) {
		return jsonschema.PropagateRef()
	}

	if strings.HasPrefix(strings.TrimSpace(f.Ref), "#") || underAdditionalProperties(f.Path) {
		return jsonschema.DropRef()
	}

	return jsonschema.PropagateRef()
}

// refMapKeywords are the schema keywords whose values are maps keyed by member
// name. A path segment immediately following one of them is a member key (a
// property or definition name), not a nested keyword.
var refMapKeywords = map[string]bool{
	jsonschema.KeywordProperties:        true,
	jsonschema.KeywordPatternProperties: true,
	jsonschema.KeywordDefs:              true,
	jsonschema.KeywordDefinitions:       true,
	jsonschema.KeywordDependentSchemas:  true,
	jsonschema.KeywordDependencies:      true,
}

// underAdditionalProperties reports whether the RFC 6901 pointer path addresses
// a schema within an additionalProperties keyword subtree. A segment equal to
// the keyword that directly follows a map keyword (e.g.
// /properties/additionalProperties) is a member name rather than the keyword,
// so it does not count; a deeper or root occurrence (e.g.
// /properties/foo/additionalProperties) does.
func underAdditionalProperties(path string) bool {
	segs := strings.Split(path, "/")
	for i := 1; i < len(segs); i++ {
		if segs[i] == jsonschema.KeywordAdditionalProperties && !refMapKeywords[segs[i-1]] {
			return true
		}
	}

	return false
}
