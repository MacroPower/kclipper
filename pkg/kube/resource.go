package kube

import "strings"

// Object represents a decoded Kubernetes resource as a map.
type Object map[string]any

// GetKind returns the kind field of the Kubernetes resource.
func (o Object) GetKind() string {
	v, ok := o["kind"].(string)
	if !ok {
		return ""
	}

	return v
}

// GetAPIVersion returns the apiVersion field of the Kubernetes resource.
func (o Object) GetAPIVersion() string {
	v, ok := o["apiVersion"].(string)
	if !ok {
		return ""
	}

	return v
}

// GetName returns the metadata.name field of the Kubernetes resource.
func (o Object) GetName() string {
	metadata, ok := o["metadata"].(map[string]any)
	if !ok {
		return ""
	}

	v, ok := metadata["name"].(string)
	if !ok {
		return ""
	}

	return v
}

// DeepCopy returns a recursive deep copy of the [Object].
func (o Object) DeepCopy() Object {
	if o == nil {
		return nil
	}

	return Object(deepCopyMap(o))
}

// ObjectsToMaps converts a slice of [Object] to a slice of plain maps.
func ObjectsToMaps(resources []Object) []map[string]any {
	result := make([]map[string]any, len(resources))
	for i, r := range resources {
		result[i] = r
	}

	return result
}

// IsCRD reports whether the [Object] is a Kubernetes CustomResourceDefinition.
// Both v1 and v1beta1 apiVersions are matched.
func (o Object) IsCRD() bool {
	return strings.HasPrefix(o.GetAPIVersion(), "apiextensions.k8s.io/") &&
		o.GetKind() == "CustomResourceDefinition"
}

func deepCopyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}

	return result
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = deepCopyValue(item)
		}

		return result

	default:
		return v
	}
}
