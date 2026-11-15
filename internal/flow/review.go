package flow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/model"
)

// Reviewing is a consumer refusing at a boundary before a person is asked:
// an agent that did not write a document reads it as the ones who will build
// from it must, and returns it to its author with what is missing. It decides
// nothing about acceptance — the criteria and the person do — and it is
// asked at most reviewRounds times, after which what it still finds goes to
// the person beside the document.
const reviewRounds = 2

// review is what a reviewing agent answers.
type review struct {
	Verdict  string `json:"verdict"`
	Findings []struct {
		Severity   string `json:"severity"`
		Where      string `json:"where"`
		Finding    string `json:"finding"`
		Suggestion string `json:"suggestion"`
	} `json:"findings"`
	Questions []string `json:"questions_for_the_person"`
}

// sufficient is whether nothing the reviewer found stops the work: no
// finding it called blocking or major.
func (v review) sufficient() bool {
	for _, f := range v.Findings {
		if f.Severity == "blocking" || f.Severity == "major" {
			return false
		}
	}
	return true
}

// forAuthor is the review as the author is handed it.
func (v review) forAuthor() string {
	var b strings.Builder
	for _, f := range v.Findings {
		fmt.Fprintf(&b, "- [%s] %s: %s", f.Severity, f.Where, f.Finding)
		if f.Suggestion != "" {
			b.WriteString(" — " + f.Suggestion)
		}
		b.WriteString("\n")
	}
	for _, q := range v.Questions {
		b.WriteString("- [question for the person] " + q + "\n")
	}
	return b.String()
}

// markdown is the review as it is kept beside the change.
func (v review) markdown(title, by string) string {
	var rows [][]string
	for _, f := range v.Findings {
		rows = append(rows, []string{f.Severity, f.Where, f.Finding, f.Suggestion})
	}
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	verdict := "sufficient: nothing found stops the work"
	if !v.sufficient() {
		verdict = "insufficient: what is marked blocking or major stops the work until it is resolved"
	}
	b.WriteString("Reviewed by " + by + ". Verdict: " + verdict + ".\n\n## Findings\n\n")
	if len(rows) == 0 {
		b.WriteString("None.\n")
	} else {
		b.WriteString("| Severity | Where | Finding | Suggestion |\n|---|---|---|---|\n")
		for _, r := range rows {
			for i := range r {
				r[i] = strings.ReplaceAll(strings.ReplaceAll(r[i], "|", `\|`), "\n", " ")
			}
			b.WriteString("| " + strings.Join(r, " | ") + " |\n")
		}
	}
	b.WriteString("\n## Questions only the person can answer\n\n")
	if len(v.Questions) == 0 {
		b.WriteString("None.\n")
	}
	for _, q := range v.Questions {
		b.WriteString("- " + q + "\n")
	}
	return b.String()
}

const reviewForm = `Answer with one JSON object and nothing else:
{"verdict": "sufficient" or "insufficient",
 "findings": [{"severity": "blocking" | "major" | "minor", "where": the section or unit, "finding": what is wrong or missing, "suggestion": how to resolve it}],
 "questions_for_the_person": [what only the person who asked for the change can decide — never guess an answer to these]}
Blocking: the work cannot be done or judged as written. Major: it would likely be built wrong. Minor: worth improving, stops nothing. Be specific; do not praise; do not repeat the document.`

const specReviewSystem = `You review a specification before anyone builds from it. You did not write it. The agents who will build it each see only their own part and never talk to anyone, so the document must stand alone.

Judge it as they will need it:
- Is every way a person uses it described, with its edge cases (wrong input, repetition, expiry, absence, refusal, concurrency)?
- Is every requirement judged by some criterion, and does every criterion judge something a person can observe, at the estate's own entry point?
- Is the design concrete enough to build without guessing: data kept, interfaces with their methods, routes or commands, which file holds what, what tests stand in for?
- Does it fit the estate as its names show, without inventing what is there or missing what must change?
- Is anything contradictory, vague, or left to taste that should be decided?

` + reviewForm

const planReviewSystem = `You review a plan that divides a specified change into units of work, before agents carry it out. You did not write it. Each unit's agent sees only its own brief, the files it may change and the code around them — never the specification or the other briefs.

Judge each unit as its agent will need it:
- Could it be carried out from its brief alone, without guessing names, signatures or behaviour?
- Do the tests units and the code units use the same names and signatures?
- Is every requirement and criterion of the specification served by some unit, and nothing done twice or by no one?
- Is the order right: does each unit have what it relies on from the units before it, and leave the estate building? A unit that uses a type or function another unit provides must come after it; two units defining the same name is blocking.
- Is any unit too large for one agent in one answer, or split where it should not be?

` + reviewForm

// reviewed asks a reviewing agent about a document and keeps what it said
// beside the change.
func (r *Run) reviewed(ctx context.Context, area int, kind, system, prompt string, round int) (review, error) {
	m := r.cfg.reviewer()
	r.say(area, "", "asking %s to review the %s, round %d", m.Name(), kind, round)
	began := time.Now()
	answer, u, err := m.Ask(ctx, system, prompt)
	r.spend(u)
	r.cost(artifact.Cost{Area: area, What: fmt.Sprintf("review of the %s, round %d", kind, round), Actor: m.Name(), Turns: 1,
		Handed: estate.Tokens(system + prompt)}, u, time.Since(began))
	var v review
	if err != nil {
		return v, err
	}
	if err := model.JSON(answer, &v); err != nil {
		return v, fmt.Errorf("the review was not one JSON object: %w", err)
	}
	dir := filepath.Join(r.dir, "reviews")
	if err := os.MkdirAll(dir, 0o755); err == nil {
		// Numbered across every round the document went through, so that a
		// review after the person sent it back does not replace one before.
		base := strings.ReplaceAll(kind, " ", "-")
		kept, _ := filepath.Glob(filepath.Join(dir, base+"-*.md"))
		name := fmt.Sprintf("%s-%d.md", base, len(kept)+1)
		_ = os.WriteFile(filepath.Join(dir, name), []byte(v.markdown(fmt.Sprintf("Review of the %s, round %d", kind, round), m.Name())), 0o644)
	}
	n := 0
	for _, f := range v.Findings {
		if f.Severity == "blocking" || f.Severity == "major" {
			n++
		}
	}
	r.say(area, "", "the reviewer found %d finding(s), %d of them stopping the work, and %d question(s) for you", len(v.Findings), n, len(v.Questions))
	_ = r.record("review", r.id("SPEC"), fmt.Sprintf("the %s reviewed, round %d: %d finding(s), %d stopping the work", kind, round, len(v.Findings), n))
	return v, nil
}
