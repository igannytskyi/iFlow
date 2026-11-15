package greeting

import "testing"

func TestGreetTheWorld(t *testing.T) {
	if got, want := Greet(""), "Hello, world!"; got != want {
		t.Errorf("Greet(\"\") = %q, want %q", got, want)
	}
}
