package httpx

import (
	"bytes"
	"encoding/json"
)

// Optional is a tri-state JSON value for partial updates:
//
//	field absent  -> Set=false
//	field null    -> Set=true,  Null=true
//	field present -> Set=true,  Null=false, Value=<decoded>
type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON implements json.Unmarshaler. It is only invoked when the key is
// present in the payload, which is what makes "absent" distinguishable.
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}
