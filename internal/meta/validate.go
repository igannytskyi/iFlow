package meta

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Problem is one way a document breaks the model: where, and what.
type Problem struct {
	Path    string // the field, as its JSON path: criteria[2].procedure
	Message string
}

func (p Problem) String() string { return p.Path + ": " + p.Message }

// idPatterns are the identifiers conventions.md defines, by the doc tag that
// names them.
var idPatterns = map[string]*regexp.Regexp{
	"INT-<nnn>":          regexp.MustCompile(`^INT-\d{3}$`),
	"SPEC-<nnn>":         regexp.MustCompile(`^SPEC-\d{3}$`),
	"CR-<nnn>-<nn>":      regexp.MustCompile(`^CR-\d{3}-\d{2}$`),
	"WU-<nnn>-<nn>":      regexp.MustCompile(`^WU-\d{3}-\d{2}$`),
	"GR-<nnn>-<nn>":      regexp.MustCompile(`^GR-\d{3}-\d{2}$`),
	"CA-<nnn>-<nn>":      regexp.MustCompile(`^CA-\d{3}-\d{2}$`),
	"EV-<nnn>-<nn>-<nn>": regexp.MustCompile(`^EV-\d{3}-\d{2}-\d{2}$`),
	"VE-<nnn>-<nn>":      regexp.MustCompile(`^VE-\d{3}-\d{2}$`),
	"RF-<nnn>-<nn>":      regexp.MustCompile(`^RF-\d{3}-\d{2}$`),
}

// Validate is every way a document breaks the model: a code that is not one
// of its values, an identifier not in its pattern, a schema version this
// program does not write. What the document means — whether every criterion
// has an arbiter — is the gate's to judge, not this.
func Validate(doc any) []Problem {
	v := reflect.ValueOf(doc)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var out []Problem
	if f := v.FieldByName("Schema"); f.IsValid() && f.Int() != int64(Version(v.Interface())) {
		out = append(out, Problem{"schema", fmt.Sprintf("version %d, and this program writes %d", f.Int(), Version(v.Interface()))})
	}
	walk(v, "", &out)
	if p, ok := v.Interface().(Plan); ok {
		out = append(out, planProblems(p)...)
	}
	return out
}

// planProblems is what a plan's own shape asks beyond its codes: a name
// declared says where and what it is; only a description may say neither.
func planProblems(p Plan) []Problem {
	var out []Problem
	for i, u := range p.Units {
		for j, pr := range u.Provides {
			at := fmt.Sprintf("units[%d].provides[%d]", i, j)
			if pr.Kind != Described && (pr.Name == "" || pr.File == "") {
				out = append(out, Problem{at, "a " + string(pr.Kind) + " names its name and its file"})
			}
			if pr.Signature == "" {
				out = append(out, Problem{at, "no signature"})
			}
		}
	}
	return out
}

func walk(v reflect.Value, path string, out *[]Problem) {
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			f := t.Field(i)
			name, omit := jsonName(f)
			p := name
			if path != "" {
				p = path + "." + name
			}
			fv := v.Field(i)
			if fv.Kind() == reflect.String {
				check(f, fv.String(), omit, p, out)
				continue
			}
			walk(fv, p, out)
		}
	case reflect.Slice:
		for i := range v.Len() {
			walk(v.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

func check(f reflect.StructField, s string, omit bool, path string, out *[]Problem) {
	code := f.Type.Name()
	if values, ok := Codes[code]; ok {
		switch {
		case s == "" && omit:
		case s == "":
			*out = append(*out, Problem{path, "empty; it is one of " + strings.Join(values, ", ")})
		case !Allowed(code, s):
			*out = append(*out, Problem{path, fmt.Sprintf("%q is not one of %s", s, strings.Join(values, ", "))})
		}
		return
	}
	if re, ok := idPatterns[f.Tag.Get("doc")]; ok && f.Name == "ID" {
		if !re.MatchString(s) {
			*out = append(*out, Problem{path, fmt.Sprintf("%q is not an identifier of the form %s", s, f.Tag.Get("doc"))})
		}
	}
}

// jsonName is a field's name in JSON, and whether it may be left out.
func jsonName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, strings.Contains(opts, "omitempty")
}
