package artifact

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CostFile is where a change keeps its cost records, one JSON object a line,
// appended and never rewritten.
const CostFile = "costs.jsonl"

// Cost is the method's CostRecord: what one piece of work consumed,
// attributed to the area and unit it was done for. Money is as the provider
// prices it, which on a subscription is the price of the same work, not a
// charge.
type Cost struct {
	At    string `json:"at"`
	Area  int    `json:"area"`
	Unit  string `json:"unit,omitempty"`
	What  string `json:"what"`
	Actor string `json:"actor"`
	Model string `json:"model,omitempty"`
	Turns int    `json:"turns,omitempty"`
	Steps int    `json:"steps,omitempty"`
	// Handed is what the framework gave the model to read, in tokens, before
	// anything the model or its provider added: the measure of how little
	// context a piece of work was given.
	Handed     int     `json:"handed,omitempty"`
	Input      int     `json:"input"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Output     int     `json:"output"`
	CostUSD    float64 `json:"cost_usd"`
	ElapsedMS  int64   `json:"elapsed_ms"`
	// Provenance is derived when the record was written by what did the
	// work, attested when it was written afterwards from what was reported.
	Provenance string `json:"provenance"`
	Note       string `json:"note,omitempty"`
}

// In is every token read, cached or not.
func (c Cost) In() int { return c.Input + c.CacheRead + c.CacheWrite }

// AppendCost adds a record to a change.
func AppendCost(dir string, c Cost) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, CostFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// Costs reads a change's records; a change with none has none.
func Costs(dir string) ([]Cost, error) {
	f, err := os.Open(filepath.Join(dir, CostFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Cost
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var c Cost
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return out, err
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// Total sums records.
func Total(cs []Cost) Cost {
	var t Cost
	for _, c := range cs {
		t.Turns += c.Turns
		t.Steps += c.Steps
		t.Input += c.Input
		t.CacheRead += c.CacheRead
		t.CacheWrite += c.CacheWrite
		t.Output += c.Output
		t.CostUSD += c.CostUSD
		t.ElapsedMS += c.ElapsedMS
	}
	return t
}
