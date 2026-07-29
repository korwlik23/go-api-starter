package openapitools

import (
	"encoding/json"
	"fmt"
)

var supportedSurfaces = map[string]struct{}{
	"admin":       {},
	"public":      {},
	"site-server": {},
}

var operationMethods = map[string]struct{}{
	"delete": {}, "get": {}, "head": {}, "options": {},
	"patch": {}, "post": {}, "put": {}, "trace": {},
}

// FilterDocument returns a deep-copied OpenAPI document containing only
// operations explicitly assigned to surface.
func FilterDocument(document map[string]any, surface string) (map[string]any, error) {
	if _, ok := supportedSurfaces[surface]; !ok {
		return nil, fmt.Errorf("unsupported surface %q", surface)
	}

	contents, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("clone OpenAPI document: %w", err)
	}
	var filtered map[string]any
	if err := json.Unmarshal(contents, &filtered); err != nil {
		return nil, fmt.Errorf("clone OpenAPI document: %w", err)
	}

	paths, ok := filtered["paths"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI document paths must be an object")
	}
	for path, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q must be an object", path)
		}
		keptOperation := false
		for method, rawOperation := range pathItem {
			if _, ok := operationMethods[method]; !ok {
				continue
			}
			operation, ok := rawOperation.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("operation %s %s must be an object", method, path)
			}
			rawSurfaces, ok := operation["x-surfaces"].([]any)
			if !ok || len(rawSurfaces) == 0 {
				return nil, fmt.Errorf("operation %s %s must declare non-empty x-surfaces", method, path)
			}
			selected := false
			for _, rawSurface := range rawSurfaces {
				name, ok := rawSurface.(string)
				if !ok {
					return nil, fmt.Errorf("operation %s %s has non-string x-surfaces value", method, path)
				}
				if name == surface {
					selected = true
				}
			}
			if !selected {
				delete(pathItem, method)
				continue
			}
			delete(operation, "x-surfaces")
			keptOperation = true
		}
		if !keptOperation {
			delete(paths, path)
		}
	}
	return filtered, nil
}
