package reader

import (
	"slices"
	"testing"
)

// Tests are read from the syntax tree as each runner finds them: what a
// pattern over the text would take — a test in a string, a comment, a
// nested helper — is not taken.
func TestTestsAreReadFromTheSyntaxTree(t *testing.T) {
	for _, c := range []struct {
		file, src string
		want      []string
	}{
		{"test_greet.py", `
import pytest

def test_greets():
    def test_nested_helper():
        pass
    assert True

@pytest.mark.parametrize("n", [1, 2])
def test_shouts(n):
    pass

class TestGreeter:
    def test_hello(self):
        pass
    def helper(self):
        pass

# def test_in_a_comment(): pass
s = "def test_in_a_string(): pass"
`, []string{"test_greets", "test_shouts", "test_hello"}},
		{"greet.test.js", "const test = require('node:test');\n" +
			"test('greets the person', () => {});\n" +
			"describe('greeting', () => {\n  it(`says goodbye`, () => {});\n});\n" +
			"// test('in a comment', () => {})\n" +
			"const s = \"test('in a string', () => {})\";\n" +
			"helper.test('not the runner', () => {});\n",
			[]string{"greets the person", "says goodbye"}},
		{"greet.test.ts", "import test from 'node:test';\ntest('typed', (): void => {});\n", []string{"typed"}},
	} {
		got, ok := Tests(c.file, []byte(c.src))
		slices.Sort(got)
		want := slices.Sorted(slices.Values(c.want))
		if !ok || !slices.Equal(got, want) {
			t.Errorf("%s: %v (known %v), want %v", c.file, got, ok, want)
		}
	}
	if _, ok := Tests("lib.rs", []byte("#[test]\nfn adds() {}\n")); ok {
		t.Error("a language the reader does not read tests of was said to be known")
	}
}
