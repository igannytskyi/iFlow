package reader

import (
	"slices"
	"testing"
)

func has(calls []Call, name string, kind Kind) bool {
	return slices.ContainsFunc(calls, func(c Call) bool { return c.Name == name && c.Kind == kind })
}

func TestPythonTypesWhatTheFileStates(t *testing.T) {
	src := `import os.path
from pkg.mod import Thing as T

class Base: pass

class Session(Base):
    def mount(self):
        self.prepare()

def use(adapter: Adapter, maybe: Optional[Other]):
    s = Session()
    s.mount()
    adapter.send()
    maybe.close()
    unknown.call()
    helper()
`
	f := Read("x.py", []byte(src))
	for _, name := range []string{"Base", "Session", "mount", "use"} {
		if !slices.ContainsFunc(f.Symbols, func(s Symbol) bool { return s.Name == name }) {
			t.Errorf("%s is not read as defined", name)
		}
	}
	for _, want := range []struct {
		name string
		kind Kind
	}{
		{"Session", Typed}, // s = Session(); s.mount()
		{"Adapter", Typed}, // an annotated parameter
		{"Other", Typed},   // the one thing inside Optional[...]
		{"mount", Attribute}, {"send", Attribute}, {"call", Attribute},
		{"helper", Name}, {"Base", Name}, // a base class is a dependency written down
		{"os.path", Import}, {"Thing", Import}, {"mod", Import},
	} {
		if !has(f.Calls, want.name, want.kind) {
			t.Errorf("no %s call to %s in %v", want.kind, want.name, f.Calls)
		}
	}
	if has(f.Calls, "unknown", Typed) {
		t.Error("a receiver nothing types was typed")
	}
}

func TestPythonThatDoesNotParseIsNotReadInPart(t *testing.T) {
	f := Read("x.py", []byte("def broken(:\n    pass\n"))
	if len(f.Symbols) != 0 || len(f.Unresolved) != 1 {
		t.Errorf("read %v, %v", f.Symbols, f.Unresolved)
	}
}

func TestGoIsReadThroughItsGrammar(t *testing.T) {
	src := `package shop

import (
	"fmt"
	"example.com/shop/cart"
)

type Order struct{}

func (o Order) Total() int { return cart.Sum(1) }

func Print() { fmt.Println(Order{}.Total()) }
`
	f := Read("shop/order.go", []byte(src))
	for _, name := range []string{"Order", "Total", "Print"} {
		if !slices.ContainsFunc(f.Symbols, func(s Symbol) bool { return s.Name == name }) {
			t.Errorf("%s is not read as defined: %v", name, f.Symbols)
		}
	}
	if !has(f.Calls, "cart", Import) || !has(f.Calls, "fmt", Import) {
		t.Errorf("the grouped imports are not each read: %v", f.Calls)
	}
	if !has(f.Calls, "Sum", Attribute) {
		t.Errorf("a call through a package is not read: %v", f.Calls)
	}
}

func TestAScriptIsReadByItsFirstLine(t *testing.T) {
	if got := Shebang([]byte("#!/usr/bin/env bash\necho hi\n")); got != ".sh" {
		t.Errorf("Shebang = %q", got)
	}
	if got := Shebang([]byte("echo hi\n")); got != "" {
		t.Errorf("a file with no first-line interpreter was given %q", got)
	}
}

func TestModulesNameTheFileAndThePlace(t *testing.T) {
	for in, want := range map[string][]string{
		`"./lib/common.sh"`:            {"common", "lib", "lib/common"},
		`using eShop.WebApp.Catalog;`:  {"Catalog", "WebApp", "eShop/WebApp/Catalog"},
		`"fmt"`:                        {"fmt"},
		`use crate::search::Searcher;`: {"Searcher", "search", "crate/search/Searcher"},
	} {
		if got := modules(in); !slices.Equal(got, want) {
			t.Errorf("modules(%s) = %v, want %v", in, got, want)
		}
	}
}
