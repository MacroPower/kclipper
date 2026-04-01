package jsonschema

import (
	"fmt"
	"strconv"
	"strings"
)

// resolveJSONPointer resolves an RFC 6901 JSON Pointer against a generic value
// (typically the result of json.Unmarshal into any). It supports traversal of
// map[string]any and []any values.
func resolveJSONPointer(data any, pointer string) (any, error) {
	if pointer == "" {
		return data, nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q: must start with /", pointer)
	}

	tokens := strings.Split(pointer[1:], "/")
	current := data

	for _, token := range tokens {
		// RFC 6901: unescape ~1 → / then ~0 → ~.
		token = strings.ReplaceAll(token, "~1", "/")
		token = strings.ReplaceAll(token, "~0", "~")

		switch v := current.(type) {
		case map[string]any:
			val, ok := v[token]
			if !ok {
				return nil, fmt.Errorf("key %q not found", token)
			}

			current = val

		case []any:
			idx, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("invalid array index %q: %w", token, err)
			}

			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds (length %d)", idx, len(v))
			}

			current = v[idx]

		default:
			return nil, fmt.Errorf("cannot traverse %T with token %q", current, token)
		}
	}

	return current, nil
}
