package events

import "encoding/json"

// jsonMarshal is a tiny indirection so the file doesn't import encoding/json
// in many places.
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
