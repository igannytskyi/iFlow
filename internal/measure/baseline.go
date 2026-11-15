// Package measure is area 13: the present way of working, measured on a real
// estate, and what the same work cost when it was carried through the
// method. Without a baseline a figure about the method is a figure about
// nothing.
//
// Three readings govern every figure here. Never blend: every figure is per
// class, with the mix beside it. The denominator is the intent: units can be
// split to improve any per-unit figure, intents cannot. What is wrongly
// accepted is a lagging, censored estimate, and the lag is an instrument.
package measure

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// says is what a message says a change was: a proxy for its class, and
// declared as one — a class is defined by how acceptance is decided, and a
// subject line says only what its author called the work.
var says = []struct {
	class string
	re    *regexp.Regexp
}{
	{"C1*", regexp.MustCompile(`\b(bump|upgrade|update deps|dependenc|refactor|rename|move|typo|format|lint|style|cleanup|remove unused|dead code|drop support)\b`)},
	{"C2", regexp.MustCompile(`\b(fix|bug|regression|broken|crash|error|fault|repair|correct)\b`)},
	{"C3", regexp.MustCompile(`\b(deprecat|api|interface|schema|contract|signature|protocol|compat|breaking|add|new|support|feature|implement|introduce|allow)\b`)},
}

func called(text string) string {
	text = strings.ToLower(text)
	for _, s := range says {
		if s.re.MatchString(text) {
			return s.class
		}
	}
	return "unclassified"
}

var (
	revertRE = regexp.MustCompile(`(?i)^revert\b|\brevert(s|ing)? commit ([0-9a-f]{7,40})`)
	prRE     = regexp.MustCompile(`\(#(\d+)\)\s*$|Merge pull request #(\d+)`)
	shaRE    = regexp.MustCompile(`\b([0-9a-f]{7,40})\b`)
	githubRE = regexp.MustCompile(`github\.com[/:]([^/]+/[^/.]+)`)
)

// Commit is one commit of an estate's history.
type Commit struct {
	SHA, Who, When, Subject, Body string
	Files                         []string
}

func git(repo string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	return string(out), err
}

// Commits is every commit, newest first, with what it says and what it touched.
func Commits(repo, since string) []Commit {
	args := []string{"log", "--format=%x1e%H%x1f%an%x1f%cI%x1f%P%x1f%s%x1f%b", "--name-only"}
	if since != "" {
		args = append(args, "--since="+since)
	}
	out, _ := git(repo, args...)
	var cs []Commit
	for block := range strings.SplitSeq(out, "\x1e") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		head, files, _ := strings.Cut(block, "\n\n")
		parts := strings.Split(head, "\x1f")
		if len(parts) < 5 {
			continue
		}
		c := Commit{SHA: parts[0], Who: parts[1], When: parts[2][:min(10, len(parts[2]))], Subject: strings.TrimSpace(parts[4])}
		if len(parts) > 5 {
			c.Body = parts[5]
		}
		for f := range strings.SplitSeq(files, "\n") {
			if strings.TrimSpace(f) != "" {
				c.Files = append(c.Files, f)
			}
		}
		cs = append(cs, c)
	}
	return cs
}

// ForgeName is which project on the forge an estate is a copy of, following
// a clone of a clone to the first remote that names one.
func ForgeName(repo string) string {
	seen := map[string]bool{}
	for range 3 {
		out, _ := git(repo, "remote", "get-url", "origin")
		remote := strings.TrimSpace(out)
		if remote == "" || seen[remote] {
			return ""
		}
		seen[remote] = true
		if m := githubRE.FindStringSubmatch(remote); m != nil {
			return m[1]
		}
		if st, err := os.Stat(remote); err != nil || !st.IsDir() {
			return ""
		}
		repo = remote
	}
	return ""
}

// PR is a merged pull request as the forge kept it.
type PR struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	MergedAt string    `json:"mergedAt"`
	Comments []any     `json:"comments"`
	Reviews  []any     `json:"reviews"`
	Commits  []any     `json:"commits"`
	Created  time.Time `json:"createdAt"`
}

// Touchpoints is every occasion on which a person had to act on a pull
// request: stating it, every review, every comment, every commit after the
// first.
func (p PR) Touchpoints() int {
	return 1 + len(p.Reviews) + len(p.Comments) + max(0, len(p.Commits)-1)
}

// Forge reads merged pull requests, thirty at a time — the largest page the
// forge's query budget allows for this question.
func Forge(repo, since string, limit int) ([]PR, string) {
	name := ForgeName(repo)
	if name == "" {
		return nil, "no github remote, so no review records to read"
	}
	var rows []PR
	seen := map[int]bool{}
	cursor := ""
	for len(rows) < limit {
		args := []string{"pr", "list", "--repo", name, "--state", "merged", "--limit", "30", "--json", "number,title,createdAt,mergedAt,comments,reviews,commits"}
		if cursor != "" {
			args = append(args, "--search", "is:merged merged:<"+cursor)
		}
		var errb bytes.Buffer
		cmd := exec.Command("gh", args...)
		cmd.Stderr = &errb
		out, err := cmd.Output()
		if err != nil {
			if len(rows) > 0 {
				break
			}
			msg := strings.TrimSpace(errb.String())
			return nil, fmt.Sprintf("the forge could not be asked (%s)", msg[:min(60, len(msg))])
		}
		var page []PR
		if json.Unmarshal(out, &page) != nil {
			break
		}
		page = slices.DeleteFunc(page, func(p PR) bool { return seen[p.Number] })
		if len(page) == 0 {
			break
		}
		cursor = page[0].MergedAt
		for _, p := range page {
			seen[p.Number] = true
			cursor = min(cursor, p.MergedAt)
		}
		rows = append(rows, page...)
		cursor = cursor[:min(10, len(cursor))]
		if since != "" && cursor < since {
			break
		}
	}
	rows = slices.DeleteFunc(rows, func(p PR) bool { return since != "" && p.MergedAt[:min(10, len(p.MergedAt))] < since })
	return rows, fmt.Sprintf("%d merged pull request(s) read from %s", len(rows), name)
}

// Revert is one acceptance the estate took back.
type Revert struct {
	SHA, At, Undid, AcceptedAt, ClassSaid string
}

// Reverted is what was taken and later taken back. Reverts and nothing else:
// a later fix says an issue existed, not that a change was wrong to accept.
func Reverted(cs []Commit) []Revert {
	var out []Revert
	for _, c := range cs {
		if !revertRE.MatchString(c.Subject) && !revertRE.MatchString(c.Body) {
			continue
		}
		r := Revert{SHA: c.SHA[:12], At: c.When}
		for _, m := range shaRE.FindAllString(c.Subject+" "+c.Body, -1) {
			if i := slices.IndexFunc(cs, func(t Commit) bool { return strings.HasPrefix(t.SHA, m) }); i >= 0 {
				t := cs[i]
				r.Undid, r.AcceptedAt, r.ClassSaid = t.SHA[:12], t.When, called(t.Subject+" "+t.Body)
				break
			}
		}
		out = append(out, r)
	}
	return out
}

// Baseline is the present way of working on one estate.
type Baseline struct {
	Estate, ReadAt, Since, Until string
	Commits, Intents             int
	Mix                          map[string]int
	MixOrder                     []string
	Touchpoints                  map[string]float64
	Reverts                      []Revert
	Lag                          []int // days from acceptance to discovery, sorted
	Strength                     Strength
	ReadFrom                     string
}

// Strength is what stands behind the criteria, per region.
type Strength struct {
	Regions, Claimed, Partly, Unclaimed, Named, Symbols int
}

// ErrNotARepository means there is no history to read: a figure over
// nothing reads the same as a figure over an estate with no history.
var ErrNotARepository = errors.New("not a repository that can be read here — nothing was measured")

// Measure reconstructs the baseline from history and what the forge kept.
func Measure(repo, since string) (*Baseline, error) {
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("%s is %w", repo, ErrNotARepository)
	}
	cs := Commits(repo, since)
	prs, note := Forge(repo, since, 400)
	b := &Baseline{Estate: filepath.Base(repo), ReadAt: strings.TrimSpace(head), Commits: len(cs), Mix: map[string]int{},
		Touchpoints: map[string]float64{}, ReadFrom: note, Since: since}
	if len(cs) > 0 {
		b.Until = cs[0].When
		if since == "" {
			b.Since = cs[len(cs)-1].When
		}
	}
	touch := map[string][]int{}
	if len(prs) > 0 {
		for _, p := range prs {
			k := called(p.Title)
			b.Mix[k]++
			touch[k] = append(touch[k], p.Touchpoints())
		}
		b.Intents = len(prs)
	} else {
		// One intent is one pull request where history names it, one commit
		// where it does not: never a unit.
		groups := map[string][]Commit{}
		var order []string
		for _, c := range cs {
			key := "commit-" + c.SHA[:12]
			if m := prRE.FindStringSubmatch(c.Subject); m != nil {
				key = "pr-" + m[1] + m[2]
			}
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], c)
		}
		for _, k := range order {
			g := groups[k]
			votes, people := map[string]int{}, map[string]bool{}
			for _, c := range g {
				votes[called(c.Subject+" "+c.Body)]++
				people[c.Who] = true
			}
			klass := ""
			for _, c := range g { // the first most common, as a counter gives it
				if k := called(c.Subject + " " + c.Body); klass == "" || votes[k] > votes[klass] {
					klass = k
				}
			}
			b.Mix[klass]++
			touch[klass] = append(touch[klass], 1+max(0, len(g)-1)+max(0, len(people)-1))
		}
		b.Intents = len(groups)
	}
	for k, v := range touch {
		sum := 0
		for _, n := range v {
			sum += n
		}
		b.Touchpoints[k] = float64(int(100*float64(sum)/float64(len(v))+0.5)) / 100
	}
	b.MixOrder = slices.SortedStableFunc(func(yield func(string) bool) {
		for k := range b.Mix {
			if !yield(k) {
				return
			}
		}
	}, func(x, y string) int { return cmp.Or(cmp.Compare(b.Mix[y], b.Mix[x]), cmp.Compare(x, y)) })
	b.Reverts = Reverted(cs)
	for _, r := range b.Reverts {
		if r.AcceptedAt == "" {
			continue
		}
		a, err1 := time.Parse(time.DateOnly, r.AcceptedAt)
		d, err2 := time.Parse(time.DateOnly, r.At)
		if err1 == nil && err2 == nil {
			b.Lag = append(b.Lag, int(d.Sub(a).Hours()/24))
		}
	}
	slices.Sort(b.Lag)
	rows, err := estate.Open(repo).EveryRegion()
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.TestGround {
			continue
		}
		b.Strength.Regions++
		b.Strength.Named += r.Named
		b.Strength.Symbols += r.Symbols
		switch r.Verdict {
		case estate.Claimed:
			b.Strength.Claimed++
		case estate.PartlyClaimed:
			b.Strength.Partly++
		default:
			b.Strength.Unclaimed++
		}
	}
	return b, nil
}

// Carried is one intent carried through the method, and what it cost.
type Carried struct {
	Estate, Intent, Class string
	PR, Issue             int
	Touchpoints           int
	Stating, Reading      int
	Escalations, Left     int
	Upstream              int // -1 where nothing upstream is named
}

var (
	pullRE  = regexp.MustCompile(`pull request #?(\d+)`)
	issueRE = regexp.MustCompile(`issue #?(\d+)`)
)

// ThroughMethod is what each intent carried through the method cost, in
// touchpoints counted the same way as the baseline counts them: stating the
// intent, reading the criteria, every escalation raised, every verdict left
// undecided or deferred for a person.
func ThroughMethod(repo string) []Carried {
	entries, _ := os.ReadDir(filepath.Join(repo, "changes"))
	var out []Carried
	for _, e := range entries {
		c, err := artifact.Load(filepath.Join(repo, "changes", e.Name()))
		if err != nil || !c.Has(artifact.Intent) {
			continue
		}
		r := Carried{Estate: filepath.Base(repo), Intent: e.Name(), Class: "unclassified", Stating: 1, Upstream: -1}
		for _, row := range c.Doc(artifact.Specification).Rows("Field", "Value") {
			if row.Cell("Field") == "Default class" && len(row.Cell("Value")) >= 2 {
				r.Class = row.Cell("Value")[:2]
			}
		}
		for _, row := range c.Doc(artifact.Admission).Rows("Reviewed before admission") {
			if row.Cell("Reviewed before admission") == "yes" {
				r.Reading = 1
			}
		}
		r.Escalations = len(c.Doc(artifact.Assurance).Rows("Standing", "Ground"))
		for _, row := range c.Doc(artifact.Assurance).Rows("Outcome", "State") {
			if row.Cell("Outcome") == "undecided" || row.Cell("State") == "deferred" {
				r.Left++
			}
		}
		statement := ""
		for _, row := range c.Doc(artifact.Intent).Rows("Field", "Value") {
			statement += " " + row.Cell("Value")
		}
		if m := pullRE.FindStringSubmatch(statement); m != nil {
			fmt.Sscan(m[1], &r.PR)
		}
		if m := issueRE.FindStringSubmatch(statement); m != nil {
			fmt.Sscan(m[1], &r.Issue)
		}
		r.Touchpoints = r.Stating + r.Reading + r.Escalations + r.Left
		out = append(out, r)
	}
	return out
}

// Upstream is what one named pull request cost, counted by the baseline's
// rule, and what its title calls it.
func Upstream(repo string, number int) (int, string, bool) {
	name := ForgeName(repo)
	if name == "" {
		return 0, "", false
	}
	out, err := exec.Command("gh", "pr", "view", fmt.Sprint(number), "--repo", name, "--json", "number,title,comments,reviews,commits").Output()
	if err != nil {
		return 0, "", false
	}
	var p PR
	if json.Unmarshal(out, &p) != nil || p.Number == 0 {
		return 0, "", false
	}
	return p.Touchpoints(), called(p.Title), true
}

// AnsweredBy is which pull request closed an issue, by the forge's own
// record; nothing here decides which change answered which report.
func AnsweredBy(repo string, issue int) int {
	name := ForgeName(repo)
	if name == "" {
		return 0
	}
	out, err := exec.Command("gh", "issue", "view", fmt.Sprint(issue), "--repo", name, "--json", "closedByPullRequestsReferences").Output()
	if err != nil {
		return 0
	}
	var v struct {
		Refs []struct {
			Number     int `json:"number"`
			Repository struct {
				Name string `json:"name"`
			} `json:"repository"`
		} `json:"closedByPullRequestsReferences"`
	}
	if json.Unmarshal(out, &v) != nil {
		return 0
	}
	short := name[strings.LastIndex(name, "/")+1:]
	refs := slices.DeleteFunc(v.Refs, func(r struct {
		Number     int `json:"number"`
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
	}) bool {
		return r.Repository.Name != short
	})
	if len(refs) != 1 {
		return 0
	}
	return refs[0].Number
}
