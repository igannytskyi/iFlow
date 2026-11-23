package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/models"
)

// TestSetFloorPersistsAndRebuilds drives the Settings panel's switch for
// the cheapest provider: setFloor(true) must write "floor_pricing": true to
// models.json under the exact key and rebuild the flow configuration the
// way a model choice does; setting it off must write false and rebuild
// again.
func TestSetFloorPersistsAndRebuilds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	models.SetFile(path)
	defer models.SetFile("")

	u := &UI{}
	rebuilds := 0
	u.rebuild = func() (flow.Config, string) { rebuilds++; return flow.Config{}, "" }

	u.setFloor(true)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("models.json after setFloor(true): %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("models.json does not read: %v", err)
	}
	raw, ok := got["floor_pricing"]
	if !ok {
		t.Fatalf("models.json holds no key \"floor_pricing\": %s", b)
	}
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		t.Fatalf("key \"floor_pricing\" does not read as a boolean: %v", err)
	}
	if !on {
		t.Fatalf("\"floor_pricing\" on disk after setFloor(true) = %s, want true", raw)
	}
	if rebuilds == 0 {
		t.Fatal("setFloor(true) did not rebuild the flow configuration")
	}
	rebuilds = 0

	u.setFloor(false)

	b, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("models.json after setFloor(false): %v", err)
	}
	got = map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("models.json does not read: %v", err)
	}
	raw, ok = got["floor_pricing"]
	if !ok {
		t.Fatalf("models.json holds no key \"floor_pricing\": %s", b)
	}
	var off bool
	if err := json.Unmarshal(raw, &off); err != nil {
		t.Fatalf("key \"floor_pricing\" does not read as a boolean: %v", err)
	}
	if off {
		t.Fatalf("\"floor_pricing\" on disk after setFloor(false) = %s, want false", raw)
	}
	if rebuilds == 0 {
		t.Fatal("setFloor(false) did not rebuild the flow configuration")
	}
}
