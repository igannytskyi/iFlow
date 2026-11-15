package ui

import (
	"fmt"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/model"
)

// money is a sum as a person reads it: cents where there are cents.
func money(usd float64) string {
	switch {
	case usd == 0:
		return "$0"
	case usd < 0.01:
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

// tokens is a count of tokens, short.
func tokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// elapsed is a duration in milliseconds, short.
func elapsed(ms int64) string {
	s := float64(ms) / 1000
	switch {
	case s >= 60:
		return fmt.Sprintf("%dm %02ds", int(s)/60, int(s)%60)
	case s >= 10:
		return fmt.Sprintf("%.0fs", s)
	}
	return fmt.Sprintf("%.1fs", s)
}

// usageLine is what a model turn or a run cost, in one line.
func usageLine(u model.Usage) string {
	parts := []string{}
	if u.Model != "" {
		parts = append(parts, u.Model)
	}
	parts = append(parts, "in "+tokens(u.In()))
	if u.CacheRead > 0 {
		parts = append(parts, tokens(u.CacheRead)+" cached")
	}
	parts = append(parts, "out "+tokens(u.Output))
	if u.CostUSD > 0 {
		parts = append(parts, money(u.CostUSD))
	}
	return strings.Join(parts, " · ")
}

// costLine is what a set of cost records came to, in one line.
func costLine(cs []artifact.Cost) string {
	if len(cs) == 0 {
		return ""
	}
	t := artifact.Total(cs)
	models := map[string]bool{}
	var names []string
	for _, c := range cs {
		for m := range strings.SplitSeq(c.Model, ", ") {
			if m != "" && !models[m] {
				models[m] = true
				names = append(names, m)
			}
		}
	}
	parts := []string{}
	if len(names) > 0 {
		parts = append(parts, strings.Join(names, ", "))
	}
	if t.In() > 0 || t.Output > 0 {
		parts = append(parts, "in "+tokens(t.In()), "out "+tokens(t.Output))
	}
	if t.Steps > 0 {
		parts = append(parts, fmt.Sprintf("%d steps", t.Steps))
	}
	parts = append(parts, money(t.CostUSD), elapsed(t.ElapsedMS))
	return strings.Join(parts, " · ")
}

// inArea is the records attributed to one area.
func inArea(cs []artifact.Cost, area int) []artifact.Cost {
	var out []artifact.Cost
	for _, c := range cs {
		if c.Area == area {
			out = append(out, c)
		}
	}
	return out
}

// forUnit is the records attributed to one unit.
func forUnit(cs []artifact.Cost, unit string) []artifact.Cost {
	var out []artifact.Cost
	for _, c := range cs {
		if c.Unit == unit {
			out = append(out, c)
		}
	}
	return out
}
