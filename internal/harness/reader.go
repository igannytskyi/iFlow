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

// NewReader reads dir, and the libraries' source under the folders given.
func NewReader(ctx context.Context, dir string, libraries ...string) Reader {
	return Reader{&workspace{dir: dir, ctx: ctx, libraries: libraries}}
}

// readOnly is the tools that only read: the estate's graph, where the
// reader has one, then its files.
var readOnly = []string{"find_definition", "find_callers", "read_file", "list_dir", "search"}

// WithGraph is the reader with the estate's model of its code to ask.
func (r Reader) WithGraph(g Graph) Reader {
	r.ws.graph = g
	return r
}

// Tools is what the model is told it may call: the graph's only where
// there is one.
func (r Reader) Tools() []model.Tool {
	return slices.DeleteFunc(specs(), func(t model.Tool) bool {
		n := t.Function.Name
		return !slices.Contains(readOnly, n) || (r.ws.graph == nil && (n == "find_definition" || n == "find_callers"))
	})
}

// Call carries out one call, and refuses any but reading.
func (r Reader) Call(call model.ToolCall) string {
	if !slices.Contains(readOnly, call.Function.Name) {
		return "refused: only reading is allowed here"
	}
	answer, _, _, _ := Loop{}.call(r.ws, call)
	return clip(answer, 12000)
}
