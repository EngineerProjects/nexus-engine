package vector

import "encoding/json"

// matchesFilter returns true when r.Metadata satisfies every predicate in filter.
// Supported predicates:
//
//	{"key": "value"}              — simple equality
//	{"key": {"$in": ["a","b"]}}  — membership test
//
// A metadata value is either a plain scalar or a JSON-encoded array string
// (multi-identity ACLs write scope_id this way - see
// rag.IngestRequest.ScopeIDs); both predicates test against every element of
// whichever representation is present, so a multi-valued record matches an
// equality/membership check the same way a single-valued one always has.
func matchesFilter(r Record, filter map[string]any) bool {
	for k, v := range filter {
		values := metadataValues(r.Metadata[k])
		switch t := v.(type) {
		case string:
			if !containsString(values, t) {
				return false
			}
		case map[string]any:
			if ins, ok := t["$in"]; ok {
				if !intersects(values, ins) {
					return false
				}
			}
		}
	}
	return true
}

// metadataValues unpacks a metadata field into the set of values it
// represents: a JSON array string decodes to its elements, anything else is
// treated as a single scalar value (including the empty string, so a
// missing/unset field never accidentally matches).
func metadataValues(raw string) []string {
	if arr, ok := decodeMetaArray(raw); ok {
		return arr
	}
	return []string{raw}
}

// decodeMetaArray reports whether raw is a JSON-encoded string array (the
// representation multi-identity metadata uses - see
// rag.IngestRequest.ScopeIDs), decoding it if so. Shared between the
// in-memory/SQLite/HNSW matcher above and pgvector_store.go's Upsert, so
// both agree on exactly what counts as "multi-valued" for a metadata field.
func decodeMetaArray(raw string) ([]string, bool) {
	if len(raw) < 2 || raw[0] != '[' {
		return nil, false
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, false
	}
	return arr, true
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func intersects(values []string, list any) bool {
	for _, v := range values {
		if inList(v, list) {
			return true
		}
	}
	return false
}

func inList(val string, list any) bool {
	switch s := list.(type) {
	case []string:
		for _, v := range s {
			if v == val {
				return true
			}
		}
	case []any:
		for _, v := range s {
			if str, ok := v.(string); ok && str == val {
				return true
			}
		}
	}
	return false
}
