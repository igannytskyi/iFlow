// Package method is the method as data: its areas, the objects that cross
// between them, the contracts at each crossing, its rules and its classes.
// All of it is read from the documents that define the method; nothing here
// restates them.
package method

import (
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// Part says where an area stands relative to a change.
type Part int

const (
	// Path areas are steps a change travels, in order.
	Path Part = iota
	// Foundation areas hold across the whole path at once.
	Foundation
)

// Param is one of an area's ten parameters, P1 to P10.
type Param struct {
	Code string
	Text string
}

// Area is one area of responsibility.
type Area struct {
	N         int
	Name      string
	From      string
	Part      Part
	Object    string
	Subject   string
	Aim       string
	Criterion string
	Why       string
	Params    []Param
}

// Object is one kind of thing the method passes between areas.
type Object struct {
	Name     string
	Group    string
	What     string
	Schema   string
	Validity string
}

// Contract is one crossing: which objects, produced by whom, consumed by
// whom, and when they cross.
type Contract struct {
	Objects   []string
	Producers []string
	Consumers []int
	When      string
}

// Rule is one rule of the method, and whether the gate checks it.
type Rule struct {
	ID         string
	Text       string
	Mechanical bool
	How        string
	Boundary   bool
}

// Class is one change class.
type Class struct {
	Code    string
	Name    string
	Arbiter string
}

// Model is the whole method.
type Model struct {
	Areas      []Area
	Objects    []Object
	Contracts  []Contract
	Invariants []string
	Rules      []Rule
	Classes    []Class
}

// Area returns the area numbered n, or nil.
func (m *Model) Area(n int) *Area {
	for i := range m.Areas {
		if m.Areas[i].N == n {
			return &m.Areas[i]
		}
	}
	return nil
}

// Rule returns the rule with the given id, or nil.
func (m *Model) Rule(id string) *Rule {
	for i := range m.Rules {
		if m.Rules[i].ID == id {
			return &m.Rules[i]
		}
	}
	return nil
}

// Object returns the object of the given name, or nil.
func (m *Model) Object(name string) *Object {
	for i := range m.Objects {
		if m.Objects[i].Name == name {
			return &m.Objects[i]
		}
	}
	return nil
}

// Produces lists the objects an area produces, by the contracts.
func (m *Model) Produces(area int) []string {
	var out []string
	for _, c := range m.Contracts {
		for _, p := range c.Producers {
			if p == strconv.Itoa(area) {
				out = append(out, c.Objects...)
			}
		}
	}
	return out
}

// Consumes lists the objects an area consumes, by the contracts.
func (m *Model) Consumes(area int) []string {
	var out []string
	for _, c := range m.Contracts {
		for _, n := range c.Consumers {
			if n == area {
				out = append(out, c.Objects...)
			}
		}
	}
	return out
}

// StageArea is the area that writes each stage of a change.
var StageArea = map[artifact.Stage]int{
	artifact.Intent:        1,
	artifact.Specification: 1,
	artifact.Plan:          2,
	artifact.Admission:     3,
	artifact.Execution:     4,
	artifact.Assurance:     5,
	artifact.Landing:       6,
}

// Paths of the documents the model is read from.
const (
	ResearchDoc = "docs/iflow.md"
	RulesDoc    = "docs/method/rules.md"
	ConvDoc     = "docs/method/conventions.md"
)

// Load reads the model from the method's documents.
func Load(texts fs.FS) (*Model, error) {
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(texts, name)
		if err != nil {
			return "", fmt.Errorf("method: %w", err)
		}
		return string(b), nil
	}
	research, err := read(ResearchDoc)
	if err != nil {
		return nil, err
	}
	rules, err := read(RulesDoc)
	if err != nil {
		return nil, err
	}
	conv, err := read(ConvDoc)
	if err != nil {
		return nil, err
	}
	m := &Model{
		Areas:      areas(research),
		Objects:    objects(research),
		Contracts:  contracts(research),
		Invariants: invariants(research),
		Rules:      parseRules(rules),
		Classes:    classes(conv),
	}
	if len(m.Areas) == 0 || len(m.Rules) == 0 {
		return nil, fmt.Errorf("method: the documents define no areas or no rules")
	}
	return m, nil
}

// section returns the text from a heading line starting with prefix to the
// next heading at the same or a higher level.
func section(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, prefix) {
			continue
		}
		level := len(l) - len(strings.TrimLeft(l, "#"))
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if h := len(lines[j]) - len(strings.TrimLeft(lines[j], "#")); h > 0 && h <= level && strings.HasPrefix(lines[j][h:], " ") {
				end = j
				break
			}
		}
		return strings.Join(lines[i+1:end], "\n")
	}
	return ""
}

var areaHead = regexp.MustCompile(`^### (\d+)\. (.+?) — \*(.+)\*\s*$`)
var labelled = regexp.MustCompile(`\*\*(Object|Subject|Aim|Criterion of resolution)\.\*\*\s*`)
var param = regexp.MustCompile(`\*\*(P\d+)\*\*\s*`)

func areas(research string) []Area {
	body := section(research, "# 6. The areas")
	var out []Area
	var cur *Area
	part := Path
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "## Part II") {
			flush()
			part = Foundation
			continue
		}
		if m := areaHead.FindStringSubmatch(l); m != nil {
			flush()
			n, _ := strconv.Atoi(m[1])
			cur = &Area{N: n, Name: m[2], From: m[3], Part: part}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(l, "*Why separate.*") || strings.HasPrefix(l, "*Why an area"):
			_, after, _ := strings.Cut(l, "*")
			_, after, _ = strings.Cut(after, "*")
			cur.Why = Plain(strings.TrimSpace(after))
		case labelled.MatchString(l):
			idx := labelled.FindAllStringSubmatchIndex(l, -1)
			for i, ix := range idx {
				end := len(l)
				if i+1 < len(idx) {
					end = idx[i+1][0]
				}
				v := Plain(strings.TrimSpace(l[ix[1]:end]))
				switch l[ix[2]:ix[3]] {
				case "Object":
					cur.Object = v
				case "Subject":
					cur.Subject = v
				case "Aim":
					cur.Aim = v
				default:
					cur.Criterion = v
				}
			}
		case param.MatchString(l):
			idx := param.FindAllStringSubmatchIndex(l, -1)
			for i, ix := range idx {
				end := len(l)
				if i+1 < len(idx) {
					end = idx[i+1][0]
				}
				cur.Params = append(cur.Params, Param{Code: l[ix[2]:ix[3]], Text: Plain(strings.TrimSpace(l[ix[1]:end]))})
			}
		}
	}
	flush()
	return out
}

func objects(research string) []Object {
	body := section(research, "# 4. Objects")
	var out []Object
	group := ""
	var chunk []string
	emit := func() {
		for _, r := range artifact.ParseDoc(strings.Join(chunk, "\n")).Written("Object", "What it is") {
			out = append(out, Object{
				Name: Plain(r.Cell("Object")), Group: group, What: Plain(r.Cell("What it is")),
				Schema: Plain(r.Cell("Schema")), Validity: Plain(r.Cell("Validity")),
			})
		}
		chunk = nil
	}
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "## ") {
			emit()
			group = strings.TrimPrefix(l, "## ")
			continue
		}
		chunk = append(chunk, l)
	}
	emit()
	return out
}

var numbered = regexp.MustCompile(`^\d+\.\s+`)

func invariants(research string) []string {
	var out []string
	for _, l := range strings.Split(section(section(research, "# 4. Objects"), "## Invariants across objects"), "\n") {
		if numbered.MatchString(l) {
			out = append(out, Plain(numbered.ReplaceAllString(l, "")))
		}
	}
	return out
}

func contracts(research string) []Contract {
	var out []Contract
	for _, r := range artifact.ParseDoc(section(research, "# 7. Contracts")).Written("Object", "Produced by", "Consumed by") {
		c := Contract{When: Plain(r.Cell("Crosses when"))}
		for _, o := range strings.Split(r.Cell("Object"), ",") {
			if o = Plain(strings.TrimSpace(o)); o != "" {
				c.Objects = append(c.Objects, o)
			}
		}
		for _, p := range strings.Split(r.Cell("Produced by"), ",") {
			c.Producers = append(c.Producers, strings.TrimSpace(p))
		}
		for _, p := range strings.Split(r.Cell("Consumed by"), ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				c.Consumers = append(c.Consumers, n)
			}
		}
		out = append(out, c)
	}
	return out
}

func parseRules(text string) []Rule {
	var out []Rule
	boundary := false
	var chunk []string
	emit := func() {
		for _, r := range artifact.ParseDoc(strings.Join(chunk, "\n")).Written("#", "Rule", "Check") {
			how := r.Cell("How")
			if how == "" {
				how = r.Cell("How it is checked, or why it is not")
			}
			out = append(out, Rule{
				ID: Plain(r.Cell("#")), Text: Plain(r.Cell("Rule")),
				Mechanical: Plain(r.Cell("Check")) == "M", How: Plain(how), Boundary: boundary,
			})
		}
		chunk = nil
	}
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "## Rules of the boundary") {
			emit()
			boundary = true
		}
		chunk = append(chunk, l)
	}
	emit()
	return out
}

func classes(conv string) []Class {
	var out []Class
	for _, r := range artifact.ParseDoc(section(conv, "## Change class")).Written("Code", "Class", "Arbiter") {
		out = append(out, Class{Code: Plain(r.Cell("Code")), Name: Plain(r.Cell("Class")), Arbiter: Plain(r.Cell("Arbiter"))})
	}
	return out
}

var (
	link     = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	emphasis = strings.NewReplacer("**", "", "`", "", "*", "")
)

// Plain is markdown inline text as a reader sees it: links by their text,
// emphasis and code marks removed.
func Plain(s string) string {
	return strings.TrimSpace(emphasis.Replace(link.ReplaceAllString(s, "$1")))
}
