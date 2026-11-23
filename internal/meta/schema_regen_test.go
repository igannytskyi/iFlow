package meta

import "testing"

// TestSpecificationSchemaHasNoReliesOn checks that the schema the estate
// itself generates from the Specification type, in memory, names no
// property relies_on, while still naming the properties the type keeps.
func TestSpecificationSchemaHasNoReliesOn(t *testing.T) {
	schema := Schema("01-specification.json", Specification{})
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("generated schema has no properties map")
	}
	if _, present := props["relies_on"]; present {
		t.Error("generated specification schema still names property relies_on")
	}
	for _, kept := range []string{
		"schema", "id", "name", "intent", "default_class",
		"criteria", "termination", "scope", "arbiters",
	} {
		if _, present := props[kept]; !present {
			t.Errorf("generated specification schema is missing kept property %s", kept)
		}
	}
}
