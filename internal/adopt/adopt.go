// Package adopt reads work specified in another spec-driven method as intent
// and criteria.
//
// What another method calls a scenario is what this one calls a criterion:
// when these conditions hold, then this must be true. That much translates.
// What does not is said item by item rather than dropped — a requirement with
// no scenario under it, a requirement removed, a success criterion with no
// condition — because a translation that silently lost a third of what it was
// given would look complete.
//
// Two things are proposed, not derived, and the specification written says
// so: how each criterion is decided, and what evidence it needs.
package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Criterion is one scenario or outcome, translated.
type Criterion struct {
	Criterion, When, Then, From string
	// Scenario criteria are decided by a machine; outcomes by observation.
	Scenario bool
}

// Lost is one item that did not translate, and why.
type Lost struct{ What, From, Why string }

// Translation is another method's change, read as this one reads work.
type Translation struct {
	Method, Name, Title, Why string
	Scope                    []string
	Criteria                 []Criterion
	Untranslated             []Lost
}

var (
	requirementRE = regexp.MustCompile(`^###\s+Requirement:\s*(.+?)\s*$`)
	scenarioRE    = regexp.MustCompile(`^####\s+Scenario:\s*(.+?)\s*$`)
	deltaRE       = regexp.MustCompile(`(?i)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\s+Requirements\s*$`)
	stepRE        = regexp.MustCompile(`(?i)^\s*[-*]\s*\*\*(WHEN|GIVEN|AND|THEN|BUT)\*\*\s*(.+?)\s*$`)
	storyRE       = regexp.MustCompile(`(?i)^###\s+User Story\s+\d+\s*[-–—:]\s*(.+?)\s*(\(Priority:.*\))?\s*$`)
	gwtRE         = regexp.MustCompile(`(?i)\*\*Given\*\*\s*(.+?),?\s*\*\*When\*\*\s*(.+?),?\s*\*\*Then\*\*\s*(.+)$`)
	frRE          = regexp.MustCompile(`^\s*[-*]\s*\*\*(FR-\d+)\*\*:?\s*(.+)$`)
	scRE          = regexp.MustCompile(`^\s*[-*]\s*\*\*(SC-\d+)\*\*:?\s*(.+)$`)
	impactPathRE  = regexp.MustCompile("`([^`\\s]+\\.[A-Za-z0-9]+)`")
)

func clean(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(s, "|", "/")), " "))
}

func read(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func title(text, fallback string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "# ") {
			_, after, found := strings.Cut(line, ":")
			if !found {
				after = strings.TrimPrefix(line, "# ")
			}
			return strings.TrimSpace(after)
		}
	}
	return fallback
}

func section(text, name string) string {
	var out []string
	inside := false
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			inside = strings.EqualFold(strings.TrimSpace(line[3:]), name)
			continue
		}
		if inside {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// OpenSpec reads an OpenSpec change: its proposal is the intent, and every
// scenario under every requirement in its delta specs is a criterion.
func OpenSpec(folder string) (Translation, error) {
	proposal := read(filepath.Join(folder, "proposal.md"))
	t := Translation{Method: "OpenSpec", Name: filepath.Base(folder), Title: clean(title(proposal, filepath.Base(folder))),
		Why: clean(section(proposal, "Why"))}
	scope := map[string]bool{}
	for _, m := range impactPathRE.FindAllStringSubmatch(section(proposal, "Impact"), -1) {
		scope[m[1]] = true
	}
	for p := range scope {
		t.Scope = append(t.Scope, p)
	}
	slices.Sort(t.Scope)
	var specs []string
	_ = filepath.WalkDir(filepath.Join(folder, "specs"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "spec.md" {
			specs = append(specs, p)
		}
		return nil
	})
	slices.Sort(specs)
	for _, spec := range specs {
		rel, _ := filepath.Rel(folder, spec)
		rel = filepath.ToSlash(rel)
		var kind, req, scen string
		var steps [][2]string
		hasScenario := false
		reqAt, scenAt := 0, 0
		closeScenario := func(at int) {
			if scen == "" {
				return
			}
			// An AND belongs to whatever came before it: to the condition
			// until a THEN is stated, and to what must be true after.
			var when, then []string
			seenThen := false
			for _, s := range steps {
				seenThen = seenThen || s[0] == "THEN"
				if seenThen {
					then = append(then, s[1])
				} else {
					when = append(when, s[1])
				}
			}
			from := fmt.Sprintf("%s:%d", rel, at)
			switch {
			case len(when) > 0 && len(then) > 0:
				t.Criteria = append(t.Criteria, Criterion{req + " — " + scen, strings.Join(when, "; "), strings.Join(then, "; "), from, true})
			case len(when) == 0:
				t.Untranslated = append(t.Untranslated, Lost{req + " — " + scen, from, "the scenario states no condition"})
			default:
				t.Untranslated = append(t.Untranslated, Lost{req + " — " + scen, from, "the scenario states nothing that must be true"})
			}
			scen, steps = "", nil
		}
		closeRequirement := func(at int) {
			if req != "" && !hasScenario {
				why := "the requirement has no scenario, so nothing says when it is met"
				switch kind {
				case "REMOVED":
					why = "the requirement was removed, which no scenario decides"
				case "RENAMED":
					why = "the requirement was renamed, which changes no behaviour"
				}
				t.Untranslated = append(t.Untranslated, Lost{req, fmt.Sprintf("%s:%d", rel, at), why})
			}
			req, hasScenario = "", false
		}
		for i, line := range strings.Split(read(spec), "\n") {
			n := i + 1
			if m := deltaRE.FindStringSubmatch(line); m != nil {
				closeScenario(or(scenAt, n))
				closeRequirement(or(reqAt, n))
				kind = strings.ToUpper(m[1])
				continue
			}
			if m := requirementRE.FindStringSubmatch(line); m != nil {
				closeScenario(or(scenAt, n))
				closeRequirement(or(reqAt, n))
				req, reqAt = clean(m[1]), n
				continue
			}
			if m := scenarioRE.FindStringSubmatch(line); m != nil {
				closeScenario(or(scenAt, n))
				scen, steps, scenAt, hasScenario = clean(m[1]), nil, n, true
				continue
			}
			if m := stepRE.FindStringSubmatch(line); m != nil && scen != "" {
				steps = append(steps, [2]string{strings.ToUpper(m[1]), clean(m[2])})
			}
		}
		closeScenario(scenAt)
		closeRequirement(reqAt)
	}
	return t, nil
}

func or(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// SpecKit reads a Spec Kit feature: its input is the intent, every
// acceptance scenario of every user story a criterion, and every success
// criterion one decided by observation. Functional requirements say what
// must exist, not when it is judged, and are named, not translated.
func SpecKit(folder string) (Translation, error) {
	text := read(filepath.Join(folder, "spec.md"))
	t := Translation{Method: "Spec Kit", Name: filepath.Base(folder), Title: clean(title(text, filepath.Base(folder)))}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "**Input**") {
			_, after, _ := strings.Cut(line, ":")
			t.Why = clean(strings.Trim(strings.TrimSpace(after), `"`))
			break
		}
	}
	story := ""
	for i, line := range strings.Split(text, "\n") {
		from := fmt.Sprintf("spec.md:%d", i+1)
		if m := storyRE.FindStringSubmatch(line); m != nil {
			story = clean(m[1])
			continue
		}
		if strings.Contains(line, "**Given**") || strings.Contains(line, "**When**") {
			if m := gwtRE.FindStringSubmatch(line); m != nil {
				name := story
				if name == "" {
					name = "unnamed story"
				}
				t.Criteria = append(t.Criteria, Criterion{fmt.Sprintf("%s — scenario at line %d", name, i+1),
					clean(m[1] + "; " + m[2]), clean(m[3]), from, true})
			} else {
				t.Untranslated = append(t.Untranslated, Lost{clip(clean(line), 120), from, "the scenario does not state given, when and then in that order"})
			}
			continue
		}
		if m := scRE.FindStringSubmatch(line); m != nil {
			t.Criteria = append(t.Criteria, Criterion{m[1] + ": " + clean(m[2]), "the feature is in use", clean(m[2]), from, false})
			continue
		}
		if m := frRE.FindStringSubmatch(line); m != nil {
			t.Untranslated = append(t.Untranslated, Lost{m[1] + ": " + clip(clean(m[2]), 100), from,
				"a functional requirement says what must exist, not when it is judged; the scenarios that judge it are the criteria"})
		}
	}
	return t, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// Write writes the translation as an intent and a specification, with what
// could not be translated said where a reader of the specification sees it.
func Write(t Translation, out string, number int) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	nnn := fmt.Sprintf("%03d", number)
	statement := t.Title
	if t.Why != "" {
		statement += " — " + clip(t.Why, 400)
	}
	intent := fmt.Sprintf("# Intent — %s\n\n| Field | Value |\n|---|---|\n| Id | INT-%s |\n| Stated by | whoever wrote %s change %s |\n| Deciding authority | the owner of the estate it belongs to |\n| Priority | not stated |\n| Deadline | not stated |\n| Statement | %s |\n| Status | open |\n",
		filepath.Base(out), nnn, t.Method, t.Name, statement)
	var rows, lost, scope []string
	for i, c := range t.Criteria {
		procedure, evidence := "human", "runtime-observation"
		if c.Scenario {
			procedure, evidence = "machine", "test-run"
		}
		rows = append(rows, fmt.Sprintf("| CR-%s-%02d | %s | %s | %s | required | %s | %s | | %s |", nnn, i+1,
			clip(c.Criterion, 200), clip(c.When, 300), clip(c.Then, 300), procedure, evidence, c.From))
	}
	for _, u := range t.Untranslated {
		lost = append(lost, fmt.Sprintf("| %s | %s | %s |", clip(u.What, 200), u.From, u.Why))
	}
	if len(lost) == 0 {
		lost = []string{"| nothing | | |"}
	}
	for _, p := range t.Scope {
		scope = append(scope, "| "+p+" | |")
	}
	if len(scope) == 0 {
		scope = []string{"| | |"}
	}
	spec := fmt.Sprintf("# Specification — %s\n\n| Field | Value |\n|---|---|\n| Id | SPEC-%s |\n| Intent | INT-%s |\n| Default class | C3 |\n\n"+
		"Translated from %s change `%s`. Each criterion is a scenario it stated, in its own words. How each is decided and what evidence it needs are proposed by the translation, not derived: a scenario is proposed as a machine-decided test run and an outcome measured in use as an observation a person reads. Change either before admission if it is wrong.\n\n"+
		"## Acceptance criteria\n\n| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold | Translated from |\n|---|---|---|---|---|---|---|---|---|\n%s\n\n"+
		"## Not translated\n\n| What | Where | Why |\n|---|---|---|\n%s\n\n"+
		"## Scope\n\n| Included | Excluded |\n|---|---|\n%s\n\n## Arbiter\n\n| Path | What it arbitrates |\n|---|---|\n| | |\n",
		filepath.Base(out), nnn, nnn, t.Method, t.Name, strings.Join(rows, "\n"), strings.Join(lost, "\n"), strings.Join(scope, "\n"))
	if err := os.WriteFile(filepath.Join(out, "00-intent.md"), []byte(intent), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "01-specification.md"), []byte(spec), 0o644)
}
