package greeting

import "testing"

func TestGreetNamesThePerson(t *testing.T) {
	for _, name := range []string{"Ada", "Linus", "Grace Hopper"} {
		if got, want := Greet(name), "Hello, "+name+"!"; got != want {
			t.Errorf("Greet(%q) = %q, want %q", name, got, want)
		}
	}
}
