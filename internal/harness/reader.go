package harness

import (
	"context"
	"slices"

	"github.com/igannytskyi/iflow/internal/model"
)

// Reader gives a model the tools to read a repository and nothing else:
// for a conversation about the work, where the model looks before it
// answers but changes nothing.
type Reader struct{ ws *workspace }

// NewReader reads dir.
func NewReader(ctx context.Context, dir string) Reader {
	return Reader{&workspace{dir: dir, ctx: ctx}}
}

var readOnly = []string{"read_file", "list_dir", "search"}

// Tools is what the model is told it may call.
func (r Reader) Tools() []model.Tool {
	return slices.DeleteFunc(specs(), func(t model.Tool) bool { return !slices.Contains(readOnly, t.Function.Name) })
}

// Call carries out one call, and refuses any but reading.
func (r Reader) Call(call model.ToolCall) string {
	if !slices.Contains(readOnly, call.Function.Name) {
		return "refused: only reading is allowed here"
	}
	answer, _, _ := Loop{}.call(r.ws, call)
	return clip(answer, 12000)
}
