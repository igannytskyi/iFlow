package ui

import (
	"os"
	"path/filepath"
	"testing"

	"gioui.org/io/key"

	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// exampleDir is a real directory with content in it, so that a project
// built on it reads the way a real one does.
func exampleDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join(gate.Home(), "examples", "greeting")
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		return t.TempDir()
	}
	return p
}

// buildUI makes a UI over n real directories: the example first, fresh
// temporary folders after. testUI makes the first project the current one.
func buildUI(t *testing.T, n int) *UI {
	t.Helper()
	if n < 2 {
		t.Fatal("shortcut tests need two or more projects")
	}
	paths := []string{exampleDir(t)}
	for i := 1; i < n; i++ {
		paths = append(paths, t.TempDir())
	}
	return testUI(t, nil, paths...)
}

// press builds the event the dispatcher is called with: a key pressed, with
// or without the platform shortcut modifier riding along.
func press(name key.Name, withShortcut bool) key.Event {
	e := key.Event{Name: name, State: key.Press}
	if withShortcut {
		e.Modifiers = key.ModShortcut
	}
	return e
}

// opened is everything the shortcuts are allowed to touch on a project.
// The whole struct compares with ==, pointer and all.
type opened struct {
	selected   string
	item       *workspace.Item
	handedView string
	foundation int
}

func openedOf(p *project) opened {
	return opened{p.selected, p.item, p.handedView, p.foundation}
}

// putOpen writes the fields the dispatcher reads directly: a task on view,
// an open object, a handover view and a foundation page. No existing
// method is used, so that the tests judge the dispatcher's own reading.
func putOpen(p *project, item *workspace.Item, handed string, foundation int) {
	p.selected = "changes/something-on-view"
	p.item = item
	p.handedView = handed
	p.foundation = foundation
}

// TestShortcutNOpensNewIntentInCurrentProject judges CR-049-01: the
// platform shortcut with N puts the current project on the new-intent view,
// reports the key handled, and touches nothing else.
func TestShortcutNOpensNewIntentInCurrentProject(t *testing.T) {
	u := buildUI(t, 2)
	cur, other := u.projects[0], u.projects[1]
	putOpen(cur, &workspace.Item{ID: "probe"}, "changes/x/steps", 7)
	putOpen(other, &workspace.Item{ID: "other"}, "changes/y", 3)
	beforeCur := openedOf(cur)
	beforeOther := openedOf(other)

	if !u.shortcut(key.Event{Name: key.Name("N"), Modifiers: key.ModShortcut, State: key.Press}) {
		t.Fatal("Cmd/Ctrl+N was not reported handled")
	}
	if cur.selected != newKey {
		t.Fatalf("after Cmd/Ctrl+N selected = %q, want newKey", cur.selected)
	}
	if now := openedOf(cur); now.item != beforeCur.item || now.handedView != beforeCur.handedView || now.foundation != beforeCur.foundation {
		t.Fatalf("Cmd/Ctrl+N moved open state: was %+v, now %+v", beforeCur, now)
	}
	if now := openedOf(other); now != beforeOther {
		t.Fatalf("Cmd/Ctrl+N moved another project: was %+v, now %+v", beforeOther, now)
	}
	if u.preferring {
		t.Error("Cmd/Ctrl+N opened the settings page")
	}

	// Pressing it again stays on the new-intent view, still handled.
	if !u.shortcut(press(key.Name("N"), true)) {
		t.Fatal("a second Cmd/Ctrl+N was not reported handled")
	}
	if cur.selected != newKey {
		t.Fatalf("after a second Cmd/Ctrl+N selected = %q, want newKey", cur.selected)
	}
}

// TestShortcutDigitsChooseNthProjectAndClearItsOpenState judges CR-049-02:
// the platform shortcut with 1..9 chooses the N-th project, clearing its
// open state the way the sidebar's project row does, while its handover
// view survives and every other project is untouched.
func TestShortcutDigitsChooseNthProjectAndClearItsOpenState(t *testing.T) {
	cases := []struct {
		digit key.Name
		count int
		pick  int
	}{
		{"1", 9, 0},
		{"2", 3, 1},
		{"9", 9, 8},
	}
	for _, tc := range cases {
		t.Run(string(tc.digit), func(t *testing.T) {
			u := buildUI(t, tc.count)
			pick := u.projects[tc.pick]
			putOpen(pick, &workspace.Item{ID: "probe"}, "changes/kept/here", 7)
			snapshots := make([]opened, len(u.projects))
			for i, p := range u.projects {
				snapshots[i] = openedOf(p)
			}

			if !u.shortcut(press(tc.digit, true)) {
				t.Fatalf("digit %s was not reported handled", tc.digit)
			}
			if u.cur != pick {
				t.Fatalf("digit %s chose a different project, want the %d-th", tc.digit, tc.pick+1)
			}
			if pick.selected != "" || pick.item != nil || pick.foundation != 0 {
				t.Fatalf("chosen project not cleared: selected %q, item %v, foundation %d", pick.selected, pick.item, pick.foundation)
			}
			if pick.handedView != "changes/kept/here" {
				t.Fatalf("chosen project's handedView = %q, want it left as it was", pick.handedView)
			}
			for i, p := range u.projects {
				if i == tc.pick {
					continue
				}
				if now := openedOf(p); now != snapshots[i] {
					t.Fatalf("project %d changed: was %+v, now %+v", i, snapshots[i], now)
				}
			}
		})
	}
}

// TestShortcutDigitOneOnCurrentProjectStillClearsOpenState judges
// CR-049-03: re-choosing the project that is already current runs the same
// sequence and clears its open state, leaving the handover view.
func TestShortcutDigitOneOnCurrentProjectStillClearsOpenState(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0] // testUI makes the first current already
	putOpen(p, &workspace.Item{ID: "probe"}, "changes/in/progress", 7)
	beforeOther := openedOf(u.projects[1])

	if !u.shortcut(press("1", true)) {
		t.Fatal("digit 1 on the current project was not reported handled")
	}
	if u.cur != p {
		t.Fatal("digit 1 moved off the already-current project")
	}
	if p.selected != "" || p.item != nil || p.foundation != 0 {
		t.Fatalf("re-choose did not clear open state: selected %q, item %v, foundation %d", p.selected, p.item, p.foundation)
	}
	if p.handedView != "changes/in/progress" {
		t.Fatalf("re-choose cleared handedView = %q, want it left as it was", p.handedView)
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("re-choose moved another project: was %+v, now %+v", beforeOther, now)
	}
}

// TestShortcutDigitBeyondProjectCountDoesNothing judges CR-049-04: a digit
// greater than the number of projects is unhandled and moves nothing.
func TestShortcutDigitBeyondProjectCountDoesNothing(t *testing.T) {
	for _, digit := range []key.Name{"5", "9"} {
		t.Run(string(digit), func(t *testing.T) {
			u := buildUI(t, 2)
			snapshots := make([]opened, len(u.projects))
			for i, p := range u.projects {
				snapshots[i] = openedOf(p)
			}

			if u.shortcut(press(digit, true)) {
				t.Fatalf("digit %s with two projects was reported handled", digit)
			}
			if u.cur != u.projects[0] {
				t.Fatalf("digit %s changed which project is current", digit)
			}
			for i, p := range u.projects {
				if now := openedOf(p); now != snapshots[i] {
					t.Fatalf("digit %s changed project %d: was %+v, now %+v", digit, i, snapshots[i], now)
				}
			}
		})
	}
}

// TestShortcutIgnoresReleaseEvents judges CR-049-05: a release of any name
// the shortcuts claim is never consumed and changes nothing.
func TestShortcutIgnoresReleaseEvents(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0]
	putOpen(p, &workspace.Item{ID: "probe"}, "changes/x", 7)
	u.preferring = true
	before := openedOf(p)
	beforeOther := openedOf(u.projects[1])

	releases := []key.Event{
		{Name: "N", Modifiers: key.ModShortcut, State: key.Release},
		{Name: "2", Modifiers: key.ModShortcut, State: key.Release},
		{Name: key.NameEscape, State: key.Release},
	}
	for _, e := range releases {
		if u.shortcut(e) {
			t.Fatalf("release of %q was reported handled", e.Name)
		}
	}
	if now := openedOf(p); now != before {
		t.Fatalf("a release changed the chosen project: was %+v, now %+v", before, now)
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("a release changed another project: was %+v, now %+v", beforeOther, now)
	}
	if !u.preferring {
		t.Error("a release closed the settings page")
	}
}

// TestEscapeClosesOpenObjectFirst judges CR-049-06: with an object open,
// Escape closes only the object, whatever modifiers ride along.
func TestEscapeClosesOpenObjectFirst(t *testing.T) {
	for _, withShortcut := range []bool{false, true} {
		t.Run("shortcut"+map[bool]string{false: "No", true: "Yes"}[withShortcut], func(t *testing.T) {
			u := buildUI(t, 2)
			p := u.projects[0]
			putOpen(p, &workspace.Item{ID: "probe"}, "changes/x/steps", 7)
			u.preferring = true
			beforeOther := openedOf(u.projects[1])

			if !u.shortcut(press(key.NameEscape, withShortcut)) {
				t.Fatalf("Escape (shortcut modifier = %v) was not reported handled", withShortcut)
			}
			if p.item != nil {
				t.Fatalf("Escape left the object open: %v", p.item)
			}
			if p.handedView != "changes/x/steps" {
				t.Fatalf("Escape changed handedView to %q, want it untouched", p.handedView)
			}
			if p.foundation != 7 {
				t.Fatalf("Escape changed foundation to %d, want it untouched", p.foundation)
			}
			if !u.preferring {
				t.Error("Escape closed the settings page along with the object")
			}
			if now := openedOf(u.projects[1]); now != beforeOther {
				t.Fatalf("Escape moved another project: was %+v, now %+v", beforeOther, now)
			}
		})
	}
}

// TestEscapeClosesHandoverOnlyWhenNoObjectOpen judges CR-049-07: with no
// object open, Escape empties the handover view and nothing else.
func TestEscapeClosesHandoverOnlyWhenNoObjectOpen(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0]
	putOpen(p, nil, "changes/agent/handover", 7)
	u.preferring = true
	beforeOther := openedOf(u.projects[1])

	if !u.shortcut(press(key.NameEscape, false)) {
		t.Fatal("Escape over a handover was not reported handled")
	}
	if p.handedView != "" {
		t.Fatalf("Escape left handedView = %q, want it emptied", p.handedView)
	}
	if p.item != nil {
		t.Fatalf("Escape opened an object: %v", p.item)
	}
	if p.foundation != 7 {
		t.Fatalf("Escape changed foundation to %d, want it untouched", p.foundation)
	}
	if !u.preferring {
		t.Error("Escape closed the settings page along with the handover")
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("Escape moved another project: was %+v, now %+v", beforeOther, now)
	}
}

// TestEscapeClosesFoundationOnlyWhenNoObjectOrHandover judges CR-049-08:
// with no object or handover open, Escape resets the foundation page and
// leaves the settings page open.
func TestEscapeClosesFoundationOnlyWhenNoObjectOrHandover(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0]
	putOpen(p, nil, "", 7)
	u.preferring = true
	beforeOther := openedOf(u.projects[1])

	if !u.shortcut(press(key.NameEscape, false)) {
		t.Fatal("Escape over the foundation page was not reported handled")
	}
	if p.foundation != 0 {
		t.Fatalf("Escape left foundation = %d, want it reset to zero", p.foundation)
	}
	if p.item != nil || p.handedView != "" {
		t.Fatalf("Escape opened object or handover: item %v, handedView %q", p.item, p.handedView)
	}
	if !u.preferring {
		t.Error("Escape closed the settings page along with the foundation page")
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("Escape moved another project: was %+v, now %+v", beforeOther, now)
	}
}

// TestEscapeClosesSettingsLast judges CR-049-09: with nothing else open,
// Escape closes the settings page, exactly one layer per press.
func TestEscapeClosesSettingsLast(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0]
	before := openedOf(p)
	beforeOther := openedOf(u.projects[1])
	u.preferring = true

	if !u.shortcut(press(key.NameEscape, false)) {
		t.Fatal("Escape over the settings page was not reported handled")
	}
	if u.preferring {
		t.Error("Escape left the settings page open")
	}
	if now := openedOf(p); now != before {
		t.Fatalf("Escape closing the settings moved the project: was %+v, now %+v", before, now)
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("Escape moved another project: was %+v, now %+v", beforeOther, now)
	}
}

// TestEscapeWithNothingOpenAndUnclaimedKeysAreUnhandled judges CR-049-10:
// Escape with nothing open and a key the shortcuts do not claim are both
// left to the handler that follows.
func TestEscapeWithNothingOpenAndUnclaimedKeysAreUnhandled(t *testing.T) {
	u := buildUI(t, 2)
	p := u.projects[0]
	before := openedOf(p)
	beforeOther := openedOf(u.projects[1])
	u.preferring = false

	if u.shortcut(press(key.NameEscape, false)) {
		t.Fatal("Escape with nothing open was reported handled")
	}
	if u.shortcut(press("a", false)) {
		t.Fatal("an unclaimed key was reported handled")
	}
	if now := openedOf(p); now != before {
		t.Fatalf("an unhandled key changed the chosen project: was %+v, now %+v", before, now)
	}
	if now := openedOf(u.projects[1]); now != beforeOther {
		t.Fatalf("an unhandled key changed another project: was %+v, now %+v", beforeOther, now)
	}
	if u.preferring {
		t.Error("an unhandled key opened the settings page")
	}
}

// TestShortcutWithNoProjectsIsUnhandled judges the empty-estate edge: with
// no project list and nothing chosen, every key is unhandled.
func TestShortcutWithNoProjectsIsUnhandled(t *testing.T) {
	u := buildUI(t, 2)
	u.projects, u.cur = nil, nil

	for _, e := range []key.Event{
		press("N", true),
		press("1", true),
		press(key.NameEscape, false),
	} {
		if u.shortcut(e) {
			t.Fatalf("key %q was handled with no projects", e.Name)
		}
	}
}

// TestCloseTopClosesOneLayerAtATime judges the one-layer-at-a-time rule the
// Escape keys ride on: item first, then handover, then foundation, then
// nothing left to close.
func TestCloseTopClosesOneLayerAtATime(t *testing.T) {
	p := &project{}
	p.item = &workspace.Item{ID: "probe"}
	p.handedView = "changes/x"
	p.foundation = 7

	if !p.closeTop() {
		t.Fatal("closeTop reported nothing closed with an object open")
	}
	if p.item != nil || p.handedView != "changes/x" || p.foundation != 7 {
		t.Fatalf("closeTop closed more than the object: item %v, handedView %q, foundation %d", p.item, p.handedView, p.foundation)
	}

	if !p.closeTop() {
		t.Fatal("closeTop reported nothing closed with a handover open")
	}
	if p.item != nil || p.handedView != "" || p.foundation != 7 {
		t.Fatalf("closeTop closed more than the handover: item %v, handedView %q, foundation %d", p.item, p.handedView, p.foundation)
	}

	if !p.closeTop() {
		t.Fatal("closeTop reported nothing closed with a foundation page open")
	}
	if p.foundation != 0 {
		t.Fatalf("closeTop left foundation = %d, want zero", p.foundation)
	}

	if p.closeTop() {
		t.Fatal("closeTop reported a closed layer when nothing was open")
	}
}
