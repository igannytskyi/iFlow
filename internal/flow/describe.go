package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/model"
)

const describeSystem = `You write, after the fact, the specification of a change that was carried out before specifications were written whole. You are given everything the change kept: what was asked, its specification's record, its plan, its admission, its execution, the files its candidates held, its verdicts and its landing. Describe the change as it was meant and as it was built, from these — not from whether its files are still in the repository, which may since have been rewritten in another language. Do not invent what they do not show; where they show nothing for a part, leave it empty. You answer with one JSON object and nothing else: {"document": {…}}, the document with exactly these keys, each as described:
- "summary": one paragraph — what changed, for whom, and why.
- "context": the problem and its motivation, and what existed before.
- "goals", "non_goals", "users", "user_stories" ("As a <user>, I want <action>, so that <outcome>"), "flows" (every path, step by step), "requirements" (one testable statement each), "non_functional", "architecture", "data_model", "interfaces" (routes, functions, signatures, errors, as the code has them), "components" (which file holds what), "technology", "edge_cases", "alternatives", "dependencies", "assumptions", "risks", "observability", "rollout", "test_strategy" (which tests judge which criteria), "success_metrics", "open_questions": points.
Where the code shows nothing for a part, leave it empty rather than guess.`

// Describe writes, after the fact, the specification of a change carried
// out before a specification was written whole: from what was asked, its
// record, and the code its scope holds now. It is recorded as written after
// the fact, by whom, and what it cost.
func Describe(ctx context.Context, cfg Config, repo, dir string) error {
	c, err := artifact.Load(dir)
	if err != nil {
		return err
	}
	meta, err := os.ReadFile(filepath.Join(dir, string(artifact.Specification)+".md"))
	if err != nil {
		return err
	}
	statement := ""
	if d := c.Doc(artifact.Intent); d != nil {
		for _, r := range d.Rows("Field", "Value") {
			if r.Cell("Field") == "Statement" {
				statement = r.Cell("Value")
			}
		}
	}
	// Everything the change kept, as it kept it: its documents, its record,
	// and the files its candidates held — the change as it was, which the
	// repository, rewritten since, may no longer show.
	var b strings.Builder
	for _, st := range []artifact.Stage{artifact.Plan, artifact.Admission, artifact.Execution, artifact.Assurance, artifact.Landing} {
		if doc, err := os.ReadFile(filepath.Join(dir, string(st)+".md")); err == nil {
			b.WriteString("\n# " + string(st) + "\n\n" + string(doc) + "\n")
		}
	}
	held := 0
	_ = filepath.WalkDir(filepath.Join(dir, "candidates"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || held > cfg.packBudget()*4 {
			return err
		}
		if strings.HasSuffix(p, ".jsonl") || strings.HasSuffix(p, ".tsv") || d.Name() == "CHANGES" {
			return nil
		}
		body, err := os.ReadFile(p)
		if err == nil && len(body) < 40_000 {
			rel, _ := filepath.Rel(dir, p)
			fmt.Fprintf(&b, "\n# %s\n\n```\n%s```\n", rel, body)
			held += len(body)
		}
		return nil
	})
	prompt := "# What was asked\n\n" + statement + "\n\n# Its specification's record\n\n" + string(meta) + b.String()
	var answer struct {
		Document document `json:"document"`
	}
	began := time.Now()
	_, u, err := model.AskJSON(ctx, cfg.Model, describeSystem, prompt, &answer)
	cost := artifact.Cost{At: time.Now().Format(time.RFC3339), Area: 1, What: "the specification, written after the fact", Actor: cfg.Model.Name(),
		Turns: 1, Handed: estate.Tokens(describeSystem + prompt), Provenance: "derived", Model: u.Model,
		Input: u.Input, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Output: u.Output, CostUSD: u.CostUSD, ElapsedMS: time.Since(began).Milliseconds()}
	if p := filepath.Join(dir, HandedDir, "after-the-fact.md"); os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		if os.WriteFile(p, []byte("# Instructions\n\n"+describeSystem+"\n\n# Handed\n\n"+prompt), 0o644) == nil {
			cost.Kept = HandedDir + "/after-the-fact.md"
		}
	}
	_ = artifact.AppendCost(dir, cost)
	if errors.Is(err, model.ErrUnreadable) {
		return fmt.Errorf("the model's answer did not read: %w", err)
	}
	if err != nil {
		return err
	}
	var crit []string
	if d := c.Doc(artifact.Specification); d != nil {
		for _, r := range d.Rows("Criterion", "When") {
			crit = append(crit, fmt.Sprintf("%s %s — when %s, then %s", r.Cell("Id"), r.Cell("Criterion"), r.Cell("When"), r.Cell("Then")))
		}
	}
	parts := append(answer.Document.parts(), artifact.Part{Title: "Acceptance criteria", Points: crit})
	doc := artifact.ChangeDoc{Name: c.Name(), Statement: statement, Summary: "Written after the fact, from what the change kept: its documents, its record and its candidates.", Parts: parts}
	if err := os.WriteFile(filepath.Join(dir, artifact.ChangeFile), []byte(doc.Markdown()), 0o644); err != nil {
		return err
	}
	return artifact.Record{Name: c.Name(), Dir: dir}.Append(artifact.Crossing{At: today(), Stage: "specification", Object: artifact.ChangeFile,
		Note: "the change described after the fact, by " + cfg.Model.Name() + ", from what the change kept"})
}
