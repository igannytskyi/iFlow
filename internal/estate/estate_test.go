package estate

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func put(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// shop is a small estate: a core module, a caller that imports it, a
// stranger that shares a name with it and imports nothing, a test, and a
// region nothing names.
func shop(t *testing.T) *Repo {
	t.Helper()
	root := t.TempDir()
	put(t, root, "shop/core.py", "def total(items):\n    return sum(items)\n")
	put(t, root, "shop/checkout.py", "from shop.core import total\n\ndef pay(items):\n    return total(items)\n")
	put(t, root, "other/stranger.py", "def report():\n    return total([1])\n")
	put(t, root, "tests/test_core.py", "from shop.core import total\n\ndef test_total():\n    assert total([1, 2]) == 3\n")
	put(t, root, "lonely/orphan.py", "def alone():\n    return 1\n")
	put(t, root, "docs/guide.md", "Run shop/core.py to see the totals.\n")
	put(t, root, "web/app.ts", "export const x = 1\n")
	put(t, root, "node_modules/lib/index.py", "def total():\n    pass\n")
	return Open(root)
}

func find(rows []Reached, file string) *Reached {
	for i := range rows {
		if rows[i].File == file {
			return &rows[i]
		}
	}
	return nil
}

func TestReachIsGradedByWhatImportsIt(t *testing.T) {
	r := shop(t)
	rows, err := r.Affects([]string{"shop/core.py"})
	if err != nil {
		t.Fatal(err)
	}
	checkout, stranger := find(rows, "shop/checkout.py"), find(rows, "other/stranger.py")
	if checkout == nil || checkout.Confidence == "low" || !checkout.ImportsIt {
		t.Errorf("a caller that imports it is not reached firmly: %+v", checkout)
	}
	if stranger == nil || stranger.Confidence != "low" || stranger.ImportsIt {
		t.Errorf("a name match that imports nothing is not graded down: %+v", stranger)
	}
	if find(rows, "node_modules/lib/index.py") != nil {
		t.Error("an installed library was read as the estate")
	}
}

func TestAMentionIsNotACall(t *testing.T) {
	r := shop(t)
	named, err := r.NamedBy([]string{"shop/core.py"})
	if err != nil {
		t.Fatal(err)
	}
	if find(named, "docs/guide.md") != nil {
		t.Error("a file no reader reads was searched for mentions")
	}
	for _, n := range named {
		if n.Confidence != "low" {
			t.Errorf("a mention was graded %s", n.Confidence)
		}
	}
}

func TestRegionsSayWhereNothingJudges(t *testing.T) {
	r := shop(t)
	rows, err := r.EveryRegion()
	if err != nil {
		t.Fatal(err)
	}
	verdict := map[string]string{}
	for _, row := range rows {
		verdict[row.Region] = row.Verdict
	}
	if verdict["lonely"] != Unclaimed {
		t.Errorf("a region nothing names is %q", verdict["lonely"])
	}
	if verdict["shop"] == Unclaimed {
		t.Error("a region a test names is unclaimed")
	}
	if rows[0].Verdict != Unclaimed {
		t.Errorf("the worst region is not first: %+v", rows[0])
	}
	if !r.TestGround("tests") || r.TestGround("shop") {
		t.Error("test ground is not told from the code under test")
	}
}

func TestWhatIsNotReadIsUnseenNotAbsent(t *testing.T) {
	r := shop(t)
	// TypeScript is read here, so nothing of this estate is unseen; a
	// language nothing reads would be counted rather than dropped.
	if n := r.Unseen()[".ts"]; n != 0 {
		t.Errorf(".ts counted unseen though a reader claims it: %d", n)
	}
	put(t, r.Root, "lib/thing.ex", "defmodule Thing do\nend\n")
	if n := Open(r.Root).Unseen()[".ex"]; n != 1 {
		t.Errorf("an unread language is not counted: %d", n)
	}
}

func TestALandingTeachesAnEdgeReadingCannotFind(t *testing.T) {
	r := shop(t)
	if find(mustAffects(t, r, "lonely/orphan.py"), "shop/core.py") != nil {
		t.Fatal("the fixture already connects them")
	}
	if _, err := r.Correct([]string{"lonely/orphan.py"}, []string{"shop/core.py"}, "a-change", "loaded by name at run time"); err != nil {
		t.Fatal(err)
	}
	got := find(mustAffects(t, Open(r.Root), "lonely/orphan.py"), "shop/core.py")
	if got == nil || got.Provenance != "observed" || got.Confidence != "high" {
		t.Errorf("what a landing taught is not answered as observed: %+v", got)
	}
}

func mustAffects(t *testing.T, r *Repo, p string) []Reached {
	t.Helper()
	rows, err := r.Affects([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestContextHandsOverTheCode(t *testing.T) {
	r := shop(t)
	c, err := r.Context("total")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Subjects) != 1 || c.Subjects[0].File != "shop/core.py" || c.Subjects[0].Start != 1 {
		t.Errorf("subjects = %+v", c.Subjects)
	}
	if !slices.ContainsFunc(c.Callers, func(s Site) bool { return s.File == "shop/checkout.py" && s.Line == 4 }) {
		t.Errorf("the call site is not quoted by its line: %+v", c.Callers)
	}
	if _, ok := c.Judges["tests/test_core.py"]; !ok {
		t.Errorf("what judges it is not named: %v", c.Judges)
	}
}

func TestTheIndexIsACacheNotACorpus(t *testing.T) {
	r := shop(t)
	if _, err := r.Index(); err != nil {
		t.Fatal(err)
	}
	for _, f := range Open(r.Root).Freshness("") {
		if f.State != "fresh" {
			t.Errorf("%s is %s right after it was derived", f.Region, f.State)
		}
	}
	put(t, r.Root, "shop/core.py", "def total(items):\n    return 0\n")
	stale := 0
	for _, f := range Open(r.Root).Freshness("") {
		if f.State == "stale" {
			stale++
		}
	}
	if stale != 1 {
		t.Errorf("%d file(s) stale after one changed", stale)
	}
}

func TestARouterMatchesTheWayARouterDoes(t *testing.T) {
	for _, tc := range []struct {
		offered, consumed string
		base, want        bool
	}{
		{"/api/orders/{}", "/api/orders/{}", false, true},
		{"/{}/{}/subscriptions", "/user/orgs", false, false},
		{"/{}", "/anything", false, false},
		{"/api/catalog/items", "/api/catalog", true, true},
		{"/api/catalog/items", "/api/catalog", false, false},
	} {
		if got := covers(tc.offered, tc.consumed, tc.base); got != tc.want {
			t.Errorf("covers(%s, %s, %v) = %v", tc.offered, tc.consumed, tc.base, got)
		}
	}
	if key, host := normalise("http", "https://orders/api/Orders/{id}?x=1"); key != "/api/orders/{}" || host != "orders" {
		t.Errorf("normalise = %q, %q", key, host)
	}
	if key, _ := normalise("http", "snuba.clusters.get_nodes"); key != "" {
		t.Errorf("a module path was read as a route: %q", key)
	}
}

// A repository over another sees the copy as it is — a file changed, a file
// added — and leaves the one it is over as it was.
func TestOverSeesTheCopyAsItIs(t *testing.T) {
	base := shop(t)
	if _, err := base.Index(); err != nil {
		t.Fatal(err)
	}
	copyRoot := t.TempDir()
	for _, rel := range base.Sources() {
		b, err := os.ReadFile(base.abs(rel))
		if err != nil {
			t.Fatal(err)
		}
		put(t, copyRoot, rel, string(b))
	}
	put(t, copyRoot, "shop/core.py", "def subtotal(items):\n    return sum(items)\n")
	put(t, copyRoot, "shop/tax.py", "def vat(x):\n    return x / 5\n")
	over := base.Over(copyRoot)
	for name, want := range map[string]bool{"subtotal": true, "vat": true, "total": false, "pay": true} {
		places, err := over.WhereDefined(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(places) > 0; got != want {
			t.Errorf("over the copy, %s defined: %v, want %v", name, got, want)
		}
	}
	if places, _ := base.WhereDefined("total"); len(places) == 0 {
		t.Error("the repository it is over lost total")
	}
	if _, err := os.Stat(filepath.Join(copyRoot, ".estate")); err == nil {
		t.Error("a repository over another kept a cache in the copy")
	}
}
