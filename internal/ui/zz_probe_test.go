package ui

import (
	"fmt"
	"runtime"
	"testing"
)

func TestZZProbe(t *testing.T) {
	u := testUI(t, nil, "/Users/ig/Downloads/Projects/iFlow")
	p := u.cur
	u.parallel = runtime.GOMAXPROCS(0)
	u.load(p)
	waitFor(t, "gate", func() bool { return int(p.checked.Load()) == len(p.ws.Changes) })
	for _, c := range p.ws.Changes {
		for _, f := range c.Report.Problems() {
			fmt.Println("PROBLEM", c.Name(), f.Rule, f.Message[:min(160, len(f.Message))])
		}
	}
}
