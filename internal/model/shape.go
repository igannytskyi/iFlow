package model

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ErrUnreadable says an answer came but could not be read as what was asked
// for: a model's failure, worth asking again, not the server's.
var ErrUnreadable = errors.New("the answer does not read as what was asked for")

// Shaped is a model that can be held to a shape — a JSON schema the server
// enforces — so that a list asked for never comes back as a string.
type Shaped interface {
	AskShaped(ctx context.Context, system, prompt, name string, schema map[string]any) (string, Usage, error)
}

// AskJSON asks a question whose answer is read into into, a pointer to a
// struct. A model that can be held to a shape is given the struct's schema;
// any other is asked plainly and its answer read as best it can be. An
// answer that does not read is ErrUnreadable.
func AskJSON(ctx context.Context, m Model, system, prompt string, into any) (string, Usage, error) {
	return AskJSONOf(ctx, m, system, prompt, into, nil)
}

// AskJSONOf is AskJSON with some of the answer's strings held to the values
// given: by the path of their field, its JSON names joined by dots through
// lists and objects ("units.criteria"). What the asker knows the only
// answers to be, the model chooses among rather than writes.
func AskJSONOf(ctx context.Context, m Model, system, prompt string, into any, among map[string][]string) (string, Usage, error) {
	var answer string
	var u Usage
	var err error
	if s, ok := m.(Shaped); ok {
		t := reflect.TypeOf(into).Elem()
		schema := Schema(t)
		for path, values := range among {
			restrict(schema, strings.Split(path, "."), values)
		}
		answer, u, err = s.AskShaped(ctx, system, prompt, strings.ToLower(t.Name()), schema)
	} else {
		answer, u, err = m.Ask(ctx, system, prompt)
	}
	if err != nil {
		return answer, u, err
	}
	if err := JSON(answer, into); err != nil {
		return answer, u, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return answer, u, nil
}

// Schema is the JSON schema of a Go type as encoding/json reads it, strict:
// every field required, nothing else allowed.
func Schema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.Pointer:
		return Schema(t.Elem())
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return map[string]any{"type": "integer"}
	case reflect.Float64, reflect.Float32:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": Schema(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		var required []string
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			props[name] = Schema(f.Type)
			required = append(required, name)
		}
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	return map[string]any{}
}

// restrict holds the strings at path in schema to values: through an
// object by a property's name, through a list to its items.
func restrict(schema map[string]any, path []string, values []string) {
	for schema["type"] == "array" {
		items, ok := schema["items"].(map[string]any)
		if !ok {
			return
		}
		schema = items
	}
	if len(path) == 0 {
		if schema["type"] == "string" && len(values) > 0 {
			schema["enum"] = values
		}
		return
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	if next, ok := props[path[0]].(map[string]any); ok {
		restrict(next, path[1:], values)
	}
}

// AskShaped implements Shaped: the schema is sent as the server's
// structured output, which it holds the model to.
func (o OpenAI) AskShaped(ctx context.Context, system, prompt, name string, schema map[string]any) (string, Usage, error) {
	if name == "" {
		name = "answer"
	}
	msgs := []Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}}
	m, u, err := o.chat(ctx, msgs, nil, map[string]any{
		"type":        "json_schema",
		"json_schema": map[string]any{"name": name, "strict": true, "schema": schema},
	})
	return m.Content, u, err
}

var _ Shaped = OpenAI{}
