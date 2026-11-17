package meta

import (
	"reflect"
	"strings"
)

// Documents names every document of the model by the file it is kept in,
// with a value of its type; the record is one crossing a line.
var Documents = []struct {
	File string
	Doc  any
}{
	{"00-intent.json", Intent{}},
	{"01-specification.json", Specification{}},
	{"02-plan.json", Plan{}},
	{"03-admission.json", Admission{}},
	{"04-execution.json", Execution{}},
	{"05-assurance.json", Assurance{}},
	{"06-landing.json", Landing{}},
	{"record.jsonl", Crossing{}},
}

// Schema is the JSON Schema of a document of the model: every field with
// its type, its description, the values a code may hold and the form an
// identifier takes; a field the model may leave out is not required, and a
// field the model does not have is refused.
func Schema(file string, doc any) map[string]any {
	s := schemaOf(reflect.TypeOf(doc), "")
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = "https://github.com/igannytskyi/iflow/docs/method/schema/" + strings.TrimSuffix(strings.TrimSuffix(file, ".json"), ".jsonl") + ".schema.json"
	s["title"] = reflect.TypeOf(doc).Name()
	if file == "record.jsonl" {
		s["description"] = "One line of record.jsonl: a stage boundary crossed. The record is appended to and never rewritten; ordinals run 1, 2, 3 … in order."
	}
	return s
}

func schemaOf(t reflect.Type, doc string) map[string]any {
	var s map[string]any
	switch t.Kind() {
	case reflect.String:
		s = map[string]any{"type": "string"}
		if values, ok := Codes[t.Name()]; ok {
			s["enum"] = values
		}
		if re, ok := idPatterns[doc]; ok {
			s["pattern"] = re.String()
		}
	case reflect.Int:
		s = map[string]any{"type": "integer", "minimum": 0}
	case reflect.Slice:
		s = map[string]any{"type": "array", "items": schemaOf(t.Elem(), "")}
	case reflect.Struct:
		props := map[string]any{}
		var required []string
		for i := range t.NumField() {
			f := t.Field(i)
			name, omit := jsonName(f)
			props[name] = schemaOf(f.Type, f.Tag.Get("doc"))
			if !omit {
				required = append(required, name)
			}
		}
		s = map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
	default:
		s = map[string]any{}
	}
	if doc != "" {
		s["description"] = doc
	}
	return s
}
