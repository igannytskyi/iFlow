package artifact

import (
	"fmt"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/meta"
)

// Settle closes an escalation a change raised, by a person's decision rather
// than by a later change: what was decided, and by whom, is kept in the
// change's assurance beside what it raised, and its record says so. The
// ledger counts it settled, by the change itself.
func Settle(dir, ref, by, decision string) error {
	decision = strings.TrimSpace(decision)
	switch {
	case decision == "":
		return fmt.Errorf("%s: an escalation is settled by a decision, and none was given", ref)
	case strings.TrimSpace(by) == "":
		return fmt.Errorf("%s: an escalation is settled by someone, and nobody was named", ref)
	}
	c, err := Load(dir)
	if err != nil {
		return err
	}
	as := c.Meta.Assurance
	if as == nil {
		return fmt.Errorf("%s raised no escalation: it has no assurance", c.Name())
	}
	name, n, ok := strings.Cut(ref, "#")
	if !ok || name != c.Name() {
		return fmt.Errorf("%s is not an escalation of %s", ref, c.Name())
	}
	raised := false
	for _, e := range as.Raised {
		raised = raised || fmt.Sprint(e.N) == n
	}
	if !raised {
		return fmt.Errorf("%s raised no escalation %s", c.Name(), ref)
	}
	for _, s := range as.Settled {
		if s.Escalation == ref {
			return fmt.Errorf("%s is settled already, by %s", ref, s.ResolvedBy)
		}
	}
	as.Settled = append(as.Settled, meta.Settlement{Escalation: ref, ResolvedBy: by + ", by a decision", Note: decision})
	if err := Save(dir, Assurance, *as); err != nil {
		return err
	}
	return AppendRecord(dir, c.Name(), meta.Crossing{At: time.Now().Format("2006-01-02"), Stage: "settled", Object: ref,
		Note: by + " decided: " + decision})
}
