package modular

import (
	"fmt"
	"sort"
)

func dependencyOrder(manifests map[string]Manifest) ([]string, error) {
	inDegree := make(map[string]int, len(manifests))
	dependents := make(map[string][]string, len(manifests))
	for id, manifest := range manifests {
		inDegree[id] = len(manifest.Dependencies)
		for _, dependency := range manifest.Dependencies {
			if _, exists := manifests[dependency]; !exists {
				return nil, fmt.Errorf(
					"%w: module %s requires missing dependency %s",
					ErrInvalidRegistry,
					id,
					dependency,
				)
			}
			dependents[dependency] = append(dependents[dependency], id)
		}
	}

	ready := make([]string, 0, len(manifests))
	for id, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(manifests))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)

		sort.Strings(dependents[id])
		for _, dependent := range dependents[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(manifests) {
		return nil, fmt.Errorf("%w: module dependency graph contains a cycle", ErrInvalidRegistry)
	}
	return order, nil
}
