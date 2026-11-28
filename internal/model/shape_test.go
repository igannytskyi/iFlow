package model

import (
	"strings"
	"testing"
)

// Where an answer does not read, the model is told where and what: the
// key and the item, what it holds and what is asked — not the decoder's
// words for its own types.
func TestWhereAnAnswerDoesNotReadIsSaid(t *testing.T) {
	type doc struct {
		Requirements []string `json:"requirements"`
	}
	type draft struct {
		Later    []string `json:"later"`
		Document doc      `json:"document"`
		Ready    bool     `json:"ready"`
	}
	for answer, want := range map[string]string{
		`{"later": [{"id": "L1", "text": "x"}]}`:                `at "later", item 1 it holds an object where a string is asked`,
		`{"document": {"requirements": ["a", {"id": "FR-1"}]}}`: `at "document", "requirements", item 2 it holds an object where a string is asked`,
		`{"ready": "yes"}`:       `at "ready" it holds a string where true or false is asked`,
		`{"later": ["a"], "": }`: `it is not well-formed JSON`,
	} {
		var d draft
		err := JSON(answer, &d)
		if got := Unreadable(err, "draft"); !strings.Contains(got, want) {
			t.Errorf("%s: %q, want %q", answer, got, want)
		}
	}
}
