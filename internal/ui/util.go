package ui

import (
	"maps"
	"slices"
	"strings"

	"gioui.org/unit"
)

func cut(s, sep string) (string, string, bool) { return strings.Cut(s, sep) }

func sortedStrings[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

func unitDp(v float32) unit.Dp { return unit.Dp(v) }
