package greeting

import "testing"

func TestGreetTrimsSurroundingWhitespaceFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{" Ada", "Hello, Ada!"},
		{"Ada ", "Hello, Ada!"},
		{" Ada ", "Hello, Ada!"},
		{"  Grace Hopper  ", "Hello, Grace Hopper!"},
		{"\tLinus\n", "Hello, Linus!"},
	}
	for _, c := range cases {
		if got := Greet(c.name); got != c.want {
			t.Errorf("Greet(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGreetTreatsWhitespaceOnlyNameAsAbsent(t *testing.T) {
	for _, name := range []string{" ", "   ", "\t", "\n", " \t\n "} {
		if got, want := Greet(name), "Hello, world!"; got != want {
			t.Errorf("Greet(%q) = %q, want %q", name, got, want)
		}
	}
}
