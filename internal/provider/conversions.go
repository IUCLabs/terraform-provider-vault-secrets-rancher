package provider

import (
	"encoding/json"
	"strings"
)

// toInt64 coerces the numeric shapes Vault returns (json.Number, float64, int)
// into an int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			f, ferr := n.Float64()
			if ferr != nil {
				return 0, false
			}
			return int64(f), true
		}
		return i, true
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// splitBackendName splits an import ID of the form "<backend>/<marker>/<name>"
// (e.g. "rancher/roles/ci") into its backend and name parts.
func splitBackendName(id, marker string) (backend, name string, ok bool) {
	sep := "/" + marker + "/"
	idx := strings.LastIndex(id, sep)
	if idx <= 0 {
		return "", "", false
	}
	backend = id[:idx]
	name = id[idx+len(sep):]
	if backend == "" || name == "" {
		return "", "", false
	}
	return backend, name, true
}
