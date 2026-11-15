package reader

import (
	"encoding/json"
	"fmt"
)

// A symbol is kept as [name, start, end] and a call as [name, kind, line,
// end]: an index holds hundreds of thousands of them, and naming every field
// of every one would be most of its size.

// MarshalJSON implements json.Marshaler.
func (s Symbol) MarshalJSON() ([]byte, error) { return json.Marshal([]any{s.Name, s.Start, s.End}) }

// UnmarshalJSON implements json.Unmarshaler.
func (s *Symbol) UnmarshalJSON(b []byte) error {
	var v []json.RawMessage
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if len(v) != 3 {
		return fmt.Errorf("reader: a symbol is three values, not %d", len(v))
	}
	return unmarshalAll(v, &s.Name, &s.Start, &s.End)
}

// MarshalJSON implements json.Marshaler.
func (c Call) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{c.Name, c.Kind, c.Line, c.End})
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Call) UnmarshalJSON(b []byte) error {
	var v []json.RawMessage
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if len(v) != 4 {
		return fmt.Errorf("reader: a call is four values, not %d", len(v))
	}
	return unmarshalAll(v, &c.Name, &c.Kind, &c.Line, &c.End)
}

func unmarshalAll(v []json.RawMessage, into ...any) error {
	for i, dst := range into {
		if err := json.Unmarshal(v[i], dst); err != nil {
			return err
		}
	}
	return nil
}
