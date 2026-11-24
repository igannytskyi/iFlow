package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
)

// The plan's units as the graph they are carried in: a line a level, the
// units on one line carried side by side, each after the units it names —
// as the plan records them, which is as the scheduler carries them. In the
// execution each says how it stands, and the one at work what its agent is
// doing now.

// levels is the units by level: a unit is one level below the deepest of
// those it waits for.
func levels(units []meta.WorkUnit) [][]meta.WorkUnit {
	depth := map[string]int{}
	var at func(u meta.WorkUnit, seen map[string]bool) int
	byID := map[string]meta.WorkUnit{}
	for _, u := range units {
		byID[u.ID] = u
	}
	at = func(u meta.WorkUnit, seen map[string]bool) int {
		if d, ok := depth[u.ID]; ok {
			return d
		}
		if seen[u.ID] {
			return 0 // a circle is no plan the gate lets stand; drawn flat
		}
		seen[u.ID] = true
		d := 0
		for _, dep := range u.DependsOn {
			if du, ok := byID[dep]; ok {
				d = max(d, at(du, seen)+1)
			}
		}
		depth[u.ID] = d
		return d
	}
	var out [][]meta.WorkUnit
	for _, u := range units {
		d := at(u, map[string]bool{})
		for len(out) <= d {
			out = append(out, nil)
		}
		out[d] = append(out[d], u)
	}
	return out
}

// short is a unit's id by its last part: WU-001-04 is 04.
func short(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// unitGraph is the plan's units as their graph, within width; s and its
// execution, where given, say how each stands.
func (m Model) unitGraph(doc protocol.ChangeDoc, s service.State, live bool, width int) []string {
	if doc.Plan == nil || len(doc.Plan.Units) == 0 {
		return nil
	}
	ended := map[string]meta.Run{}
	if doc.Execution != nil {
		for _, r := range doc.Execution.Runs {
			ended[r.Unit] = r
		}
	}
	sign := func(id string) string {
		r, ok := ended[id]
		switch {
		case ok && (r.Terminal == meta.CandidateEnd || r.Candidate != ""):
			return m.st.good.Render("✓")
		case ok:
			return m.st.bad.Render("✗")
		case live && len(s.Agent[id]) > 0:
			return m.st.active.Render("●")
		}
		return m.st.faint.Render("○")
	}
	var out []string
	for n, level := range levels(doc.Plan.Units) {
		var nodes []string
		for _, u := range level {
			node := fmt.Sprintf("%s %s %s", sign(u.ID), short(u.ID), u.Kind)
			if len(u.DependsOn) > 0 {
				var after []string
				for _, d := range u.DependsOn {
					after = append(after, short(d))
				}
				node += m.st.faint.Render(" ←" + strings.Join(after, ","))
			}
			nodes = append(nodes, node)
		}
		prefix := m.st.faint.Render(fmt.Sprintf("%d ", n+1))
		out = append(out, fold(prefix+strings.Join(nodes, m.st.faint.Render("   ")), width, "  ")...)
	}
	// The unit at work, and what its agent did last.
	if live {
		for _, u := range doc.Plan.Units {
			events := s.Agent[u.ID]
			if _, done := ended[u.ID]; done || len(events) == 0 {
				continue
			}
			last := events[len(events)-1]
			what := oneLine(cmp.Or(last.Tool, string(last.Kind)) + " " + last.Detail)
			out = append(out, fold(m.st.active.Render("● "+short(u.ID))+m.st.faint.Render(" now: ")+printable(what), width, "    ")...)
		}
	}
	return append(out, "")
}

// graphRows is -1 for each line of the graph: it is read, not clicked.
func graphRows(n int) []int {
	return slices.Repeat([]int{-1}, n)
}
