// Package tui is the path in a terminal: the person's projects and their
// tasks, each task area by area, its costs, and a conversation beside it —
// what the window shows, carried by the service, read and answered from the
// keyboard. It holds nothing but what the service told it.
package tui

import (
	"context"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
)

// areaNames are the six areas of the path, as the screen names them.
var areaNames = [7]string{"", "spec", "plan", "admit", "execute", "assure", "land"}

// focus is where the keys go.
type focus int

const (
	onList focus = iota // the projects and their tasks
	onTask              // the task on view, area by area
	onChat              // the conversation beside it
)

// typing is what the line at the foot is being written for.
type typing int

const (
	reading typing = iota
	intent         // a new task, to start
	ground         // why the person does not accept, to send it back with
	asking         // words to the model, in the conversation
)

// Model is the screen.
type Model struct {
	ctx     context.Context
	client  *service.Client
	moved   chan string
	project string // the project the screen was opened on
	term    terminal

	width, height int

	projects []protocol.Project // the person's projects and their changes, as the service last said
	proj     int                // the project on view
	runs     []string           // every run watched, oldest first
	sel      int                // the task of the project on view

	focus  focus
	back   focus // where esc returns from the conversation
	area   int   // the area of the task on view, 1 to 6
	row    int   // the line chosen in the area: a unit, a verdict
	scroll int   // how far the area is scrolled

	// docs are the changes on view as their folders hold them, by folder;
	// steps how many steps their run had told when each was read.
	docs   map[string]protocol.ChangeDoc
	steps  map[string]int
	traces map[string][]protocol.Event // a candidate's agent's steps, by folder and candidate
	md     map[string]string           // a change described, drawn for a width

	input textinput.Model
	mode  typing
	note  string // what the screen last has to say: an error, an answer

	// told is, per run, the last thing the person was notified of, so that
	// nothing is notified twice.
	told map[string]string
}

// New is the screen opened on project, speaking to the service through
// client; moved carries what moved: a run's id, or a conversation's key.
func New(ctx context.Context, client *service.Client, moved chan string, project string) Model {
	in := textinput.New()
	in.CharLimit = 4000
	in.Cursor.SetMode(cursor.CursorStatic) // a cursor that does not blink asks nothing of the screen
	return Model{ctx: ctx, client: client, moved: moved, project: project, term: detect(os.Getenv), area: 1,
		input: in, told: map[string]string{}, docs: map[string]protocol.ChangeDoc{}, steps: map[string]int{},
		traces: map[string][]protocol.Event{}, md: map[string]string{}}
}

type (
	movedMsg    string
	runsMsg     []protocol.RunSummary
	projectsMsg []protocol.Project
	docMsg      protocol.ChangeDoc
	traceMsg    struct {
		key    string
		events []protocol.Event
	}
	noteMsg  string
	lostMsg  struct{}
	everyMsg struct{}
)

// Init asks for the projects and the runs, and watches every run.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.listProjects, m.listRuns, m.waitMoved, m.waitLost, every())
}

// every asks for the projects again now and then: a change carried by
// another program moves its folder, not a run this screen watches.
func every() tea.Cmd { return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return everyMsg{} }) }

func (m Model) listProjects() tea.Msg {
	ps, err := m.client.Projects(m.ctx, m.project)
	if err != nil {
		return noteMsg("the service could not list the projects: " + err.Error())
	}
	return projectsMsg(ps)
}

func (m Model) listRuns() tea.Msg {
	runs, err := m.client.Runs(m.ctx)
	if err != nil {
		return noteMsg("the service could not list its runs: " + err.Error())
	}
	for _, r := range runs {
		if err := m.client.Watch(m.ctx, r.Run); err != nil {
			return noteMsg(err.Error())
		}
	}
	return runsMsg(runs)
}

func (m Model) readDoc(dir string) tea.Cmd {
	return func() tea.Msg {
		doc, err := m.client.Change(m.ctx, dir)
		if err != nil {
			return noteMsg("the change could not be read: " + err.Error())
		}
		return docMsg(doc)
	}
}

func (m Model) readTrace(dir, candidate string) tea.Cmd {
	return func() tea.Msg {
		events, err := m.client.Trace(m.ctx, dir, candidate)
		if err != nil {
			return noteMsg("the agent's steps could not be read: " + err.Error())
		}
		return traceMsg{dir + "/" + candidate, events}
	}
}

func (m Model) waitMoved() tea.Msg { return movedMsg(<-m.moved) }

func (m Model) waitLost() tea.Msg {
	<-m.client.Done()
	return lostMsg{}
}

// task is one line under a project: a change on disk, a run carrying it, or
// both.
type task struct {
	run    string
	change protocol.Change
}

// tasks is the project's tasks: runs whose change is not yet written, then
// its changes newest first, each with the run carrying it, if one is.
func (m Model) tasks(p protocol.Project) []task {
	var out []task
	byDir := map[string]string{}
	for _, id := range m.runs {
		s, ok := m.client.Run(id)
		if !ok || s.Project != p.Path {
			continue
		}
		if s.Dir == "" || !slices.ContainsFunc(p.Changes, func(c protocol.Change) bool { return c.Dir == s.Dir }) {
			out = append(out, task{run: id, change: protocol.Change{Dir: s.Dir, Statement: s.Statement}})
			continue
		}
		byDir[s.Dir] = id // the latest run of a change stands for it
	}
	slices.Reverse(out) // the newest first
	for _, c := range p.Changes {
		out = append(out, task{run: byDir[c.Dir], change: c})
	}
	return out
}

// current is the project and the task on view, if there are any.
func (m Model) current() (protocol.Project, task, bool) {
	if m.proj >= len(m.projects) {
		return protocol.Project{}, task{}, false
	}
	p := m.projects[m.proj]
	ts := m.tasks(p)
	if len(ts) == 0 {
		return p, task{}, false
	}
	return p, ts[min(m.sel, len(ts)-1)], true
}

// dirOf is the folder of the task: its change's, or what its run has
// written to so far.
func (m Model) dirOf(t task) string {
	if s, ok := m.client.Run(t.run); ok && s.Dir != "" {
		return s.Dir
	}
	return t.change.Dir
}

// onView asks for what the task on view needs that is not read yet: its
// change as its folder holds it, read again when its run has moved.
func (m Model) onView() tea.Cmd {
	_, t, ok := m.current()
	if !ok {
		return nil
	}
	dir := m.dirOf(t)
	if dir == "" {
		return nil
	}
	s, live := m.client.Run(t.run)
	_, have := m.docs[dir]
	if !have || (live && len(s.Steps) != m.steps[dir]) {
		m.steps[dir] = len(s.Steps)
		return m.readDoc(dir)
	}
	return nil
}

// Update applies what happened: a key, a run that moved, a window resized.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(10, msg.Width-24)
		m.md = map[string]string{}
	case projectsMsg:
		first := len(m.projects) == 0
		m.projects = msg
		if first {
			m.proj = max(0, slices.IndexFunc(m.projects, func(p protocol.Project) bool { return p.Path == m.project }))
		}
		m.proj = min(m.proj, max(0, len(m.projects)-1))
		return m, m.onView()
	case runsMsg:
		for _, r := range msg {
			if !slices.Contains(m.runs, r.Run) {
				m.runs = append(m.runs, r.Run)
			}
		}
	case movedMsg:
		what := string(msg)
		cmds := []tea.Cmd{m.waitMoved}
		if !strings.HasPrefix(what, "chat:") {
			cmds = append(cmds, m.notify(what))
			if !slices.Contains(m.runs, what) {
				m.runs = append(m.runs, what)
			}
			// A run that wrote its folder, or ended, has moved what the
			// projects list.
			if s, ok := m.client.Run(what); ok && (s.Finished || s.Dir != "") {
				cmds = append(cmds, m.listProjects)
			}
			cmds = append(cmds, m.onView())
		}
		return m, tea.Batch(cmds...)
	case docMsg:
		m.docs[msg.Change.Dir] = protocol.ChangeDoc(msg)
		for k := range m.md {
			if strings.HasPrefix(k, msg.Change.Dir+"@") {
				delete(m.md, k)
			}
		}
		return m, m.traceOnView()
	case traceMsg:
		m.traces[msg.key] = msg.events
	case everyMsg:
		return m, tea.Batch(m.listProjects, every())
	case noteMsg:
		m.note = string(msg)
	case lostMsg:
		m.note = "the service has gone; the runs it carried stopped with it"
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

// notify tells the person, through the terminal, what they are waited for
// or how a run ended — once each.
func (m Model) notify(run string) tea.Cmd {
	s, ok := m.client.Run(run)
	if !ok || m.term.notify == nil {
		return nil
	}
	var what string
	switch {
	case s.Question != "":
		what = "waiting for you: " + s.Question
	case s.Finished && s.Areas[6] == protocol.Done:
		what = "landed"
	case s.Finished:
		what = "stopped"
	}
	if what == "" || m.told[run] == what {
		return nil
	}
	m.told[run] = what
	seq := m.term.notify("iflow — "+what, s.Statement)
	return func() tea.Msg {
		// One write, so that the terminal reads the sequence whole between
		// two of the screen's frames.
		_, _ = os.Stdout.WriteString(seq)
		return nil
	}
}

// begin begins typing at the foot, for what.
func (m Model) begin(what typing, prompt string) (Model, tea.Cmd) {
	m.mode, m.input.Prompt = what, prompt
	return m, m.input.Focus()
}

func (m Model) typed(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEsc:
		if m.mode == asking {
			m.focus = m.back
		}
		m.mode = reading
		m.input.Blur()
		m.input.SetValue("")
		return m, nil
	case tea.KeyEnter:
		text := strings.TrimSpace(m.input.Value())
		m.input.SetValue("")
		if text == "" {
			return m, nil
		}
		p, t, _ := m.current()
		switch m.mode {
		case intent:
			m.mode, m.sel, m.focus, m.area = reading, 0, onTask, 1 // the new task comes first, and is shown
			m.input.Blur()
			return m, m.start(p.Path, text)
		case ground:
			m.mode = reading
			m.input.Blur()
			run := t.run
			return m, func() tea.Msg { return result(m.client.Refuse(m.ctx, run, text), "sent back") }
		case asking:
			ref := m.chatRef()
			return m, func() tea.Msg { return result(m.client.Ask(m.ctx, ref, text), "") }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	return m, cmd
}

func (m Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode != reading {
		return m.typed(k)
	}
	p, t, has := m.current()
	s, live := m.client.Run(t.run)
	// What the keys do wherever the focus is.
	switch k.String() {
	case "ctrl+c", "q":
		return m, tea.Quit // the runs go on in the service
	case "n":
		if len(m.projects) > 0 {
			return m.begin(intent, "intent for "+p.Name+" ▸ ")
		}
		return m, nil
	case "c":
		if len(m.projects) == 0 {
			return m, nil
		}
		m.back, m.focus = m.focus, onChat
		ref := m.chatRef()
		next, cmd := m.begin(asking, "ask ▸ ")
		return next, tea.Batch(cmd, func() tea.Msg { return result(m.client.Talk(m.ctx, ref), "") })
	case "a":
		if live && s.Question != "" {
			run := t.run
			return m, func() tea.Msg { return result(m.client.Accept(m.ctx, run), "accepted") }
		}
	case "r":
		if live && s.Question != "" {
			return m.begin(ground, "why ▸ ")
		}
	case "s":
		if live && !s.Finished {
			run := t.run
			return m, func() tea.Msg { return result(m.client.Stop(m.ctx, run), "asked to stop") }
		}
	case "R":
		if has && !(live && !s.Finished) && t.change.State == protocol.Stopped {
			dir := t.change.Dir
			return m, func() tea.Msg {
				run, err := m.client.Resume(m.ctx, protocol.ResumeParams{Dir: dir})
				if err != nil {
					return noteMsg("it could not be resumed: " + err.Error())
				}
				return movedMsg(run)
			}
		}
	case "o":
		// The change described is read here, in the terminal: the
		// specification's area, from its top.
		if has {
			m.focus, m.area, m.row, m.scroll = onTask, 1, 0, 0
			return m, m.onView()
		}
	}
	if m.focus == onList {
		return m.listKey(k, p)
	}
	return m.taskKey(k, t)
}

// listKey moves among the projects and their tasks, and opens one.
func (m Model) listKey(k tea.KeyMsg, p protocol.Project) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "down", "j":
		m.sel = min(m.sel+1, max(0, len(m.tasks(p))-1))
	case "up", "k":
		m.sel = max(m.sel-1, 0)
	case "right", "l", "tab":
		if len(m.projects) > 0 {
			m.proj, m.sel = (m.proj+1)%len(m.projects), 0
		}
	case "left", "h", "shift+tab":
		if len(m.projects) > 0 {
			m.proj, m.sel = (m.proj+len(m.projects)-1)%len(m.projects), 0
		}
	case "enter":
		_, t, ok := m.current()
		if !ok {
			return m, nil
		}
		m.focus, m.row, m.scroll = onTask, 0, 0
		m.area = m.areaOf(t)
	}
	return m, m.onView()
}

// areaOf is the area a task opens on: where its run is, or how far it went.
func (m Model) areaOf(t task) int {
	if s, ok := m.client.Run(t.run); ok {
		for n := 1; n <= 6; n++ {
			if s.Areas[n] == protocol.Working || s.Areas[n] == protocol.Waiting || s.Areas[n] == protocol.Failed {
				return n
			}
		}
	}
	if t.change.State == protocol.Stopped {
		return min(t.change.Reached+1, 6)
	}
	return max(1, min(t.change.Reached, 6))
}

// taskKey moves within the task on view: between its areas, and along what
// each area holds.
func (m Model) taskKey(k tea.KeyMsg, t task) (tea.Model, tea.Cmd) {
	switch key := k.String(); key {
	case "esc", "backspace":
		if m.focus == onChat {
			m.focus = m.back
		} else {
			m.focus = onList
		}
	case "1", "2", "3", "4", "5", "6":
		m.area, m.row, m.scroll = int(key[0]-'0'), 0, 0
	case "right", "l", "tab":
		m.area, m.row, m.scroll = m.area%6+1, 0, 0
	case "left", "h", "shift+tab":
		m.area, m.row, m.scroll = (m.area+4)%6+1, 0, 0
	case "down", "j":
		m.row++
		m.scroll++
	case "up", "k":
		m.row = max(m.row-1, 0)
		m.scroll = max(m.scroll-1, 0)
	case "pgdown", " ":
		m.scroll += max(1, m.height/2)
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.height/2))
	}
	m.row = min(m.row, max(0, m.rows(t)-1))
	return m, tea.Batch(m.onView(), m.traceOnView())
}

func (m Model) start(project, statement string) tea.Cmd {
	return func() tea.Msg {
		run, err := m.client.Start(m.ctx, protocol.StartParams{Project: project, Statement: statement})
		if err != nil {
			return noteMsg("the task could not be started: " + err.Error())
		}
		return movedMsg(run)
	}
}

func result(err error, done string) tea.Msg {
	if err != nil {
		return noteMsg(err.Error())
	}
	if done == "" {
		return nil
	}
	return noteMsg(done)
}

var (
	faint   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8a8a85", Dark: "#7a7a75"})
	strong  = lipgloss.NewStyle().Bold(true)
	good    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#3b6d11", Dark: "#97c459"})
	bad     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#a32d2d", Dark: "#f09595"})
	active  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#185fa5", Dark: "#85b7eb"})
	waiting = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#854f0b", Dark: "#fac775"})
	chosen  = lipgloss.NewStyle().Reverse(true)
	rule    = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#d3d1c7", Dark: "#444441"})
)

// View draws the screen: the projects and their tasks on the left, the
// task on view in the middle, what it cost on the right, and the line to
// type on at the foot.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	top := strong.Render("iflow") + faint.Render(" · "+m.term.name)
	if m.note != "" {
		top += faint.Render("  ·  ") + ansi.Truncate(m.note, m.width-20, "…")
	}
	body := max(4, m.height-3)
	side := min(36, max(22, m.width/4))
	rest := m.width - side - 3
	costs := 0
	if rest >= 110 {
		costs = 30
		rest -= costs + 3
	}
	cols := []string{
		box(side, body, m.sidebar(side, body)),
		faint.Render(" │ "),
		box(rest, body, m.middle(rest, body)),
	}
	if costs > 0 {
		cols = append(cols, faint.Render(" │ "), box(costs, body, m.costs(costs)))
	}
	return top + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n" + m.foot()
}

// box is text in a box of width and height, cut where it overflows.
func box(width, height int, text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, width, "…")
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

// middle is the conversation where it is open, else the task on view.
func (m Model) middle(width, height int) string {
	if m.focus == onChat {
		return m.chat(width, height)
	}
	return m.task(width, height)
}

// mark is how a run stands, in one sign.
func mark(s service.State) string {
	switch {
	case s.Question != "":
		return waiting.Render("?")
	case s.Finished && s.Areas[6] == protocol.Done:
		return good.Render("✓")
	case s.Finished:
		return bad.Render("✗")
	}
	return active.Render("●")
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// foot is the line the person types on, or what the keys do here.
func (m Model) foot() string {
	if m.mode != reading {
		return m.input.View()
	}
	var keys string
	switch m.focus {
	case onList:
		keys = "↑↓ task · ←→ project · enter open · n new · c chat · q leave (runs go on)"
	case onTask:
		keys = "1–6 or ←→ area · ↑↓ choose/scroll · esc back · a accept · r send back · s stop · o read the spec · c chat"
	case onChat:
		keys = "esc back"
	}
	return ansi.Truncate(faint.Render(keys), m.width, "…")
}
