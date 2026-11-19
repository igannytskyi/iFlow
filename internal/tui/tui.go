// Package tui is the path in a terminal: the same runs the window shows,
// carried by the service, watched and answered from the keyboard. It holds
// nothing but what the service told it.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// typing is what the line at the foot is being written for.
type typing int

const (
	reading typing = iota
	intent         // a new task, to start
	ground         // why the person does not accept, to send it back with
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
	unit     int                // the unit whose agent is on view
	// records are the records of changes on view, by folder, as read.
	records map[string][]protocol.Crossing

	input textinput.Model
	mode  typing
	note  string // what the screen last has to say: an error, an answer

	// told is, per run, the last thing the person was notified of, so that
	// nothing is notified twice.
	told map[string]string
}

// New is the screen opened on project, speaking to the service through
// client; moved carries the ids of the runs that moved.
func New(ctx context.Context, client *service.Client, moved chan string, project string) Model {
	in := textinput.New()
	in.Prompt = "intent ▸ "
	in.CharLimit = 2000
	in.Cursor.SetMode(cursor.CursorStatic) // a cursor that does not blink asks nothing of the screen
	return Model{ctx: ctx, client: client, moved: moved, project: project, term: detect(os.Getenv),
		input: in, told: map[string]string{}, records: map[string][]protocol.Crossing{}}
}

type (
	movedMsg    string
	runsMsg     []protocol.RunSummary
	projectsMsg []protocol.Project
	recordMsg   struct {
		dir string
		rec []protocol.Crossing
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

func (m Model) loadRecord(dir string) tea.Cmd {
	return func() tea.Msg {
		rec, err := m.client.Record(m.ctx, dir)
		if err != nil {
			return noteMsg("the record could not be read: " + err.Error())
		}
		return recordMsg{dir, rec}
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

// Update applies what happened: a key, a run that moved, a window resized.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(10, msg.Width-12)
	case projectsMsg:
		first := len(m.projects) == 0
		m.projects = msg
		if first {
			m.proj = max(0, slices.IndexFunc(m.projects, func(p protocol.Project) bool { return p.Path == m.project }))
		}
		m.proj = min(m.proj, max(0, len(m.projects)-1))
		return m, m.recordOnView()
	case runsMsg:
		for _, r := range msg {
			if !slices.Contains(m.runs, r.Run) {
				m.runs = append(m.runs, r.Run)
			}
		}
	case movedMsg:
		run := string(msg)
		cmds := []tea.Cmd{m.waitMoved, m.notify(run)}
		if !slices.Contains(m.runs, run) {
			m.runs = append(m.runs, run)
		}
		// A run that wrote its folder, or ended, has moved what the projects
		// list: they are asked for again.
		if s, ok := m.client.Run(run); ok && (s.Finished || s.Dir != "") {
			cmds = append(cmds, m.listProjects)
		}
		return m, tea.Batch(cmds...)
	case recordMsg:
		m.records[msg.dir] = msg.rec
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

// recordOnView reads the record of the change on view, where no run tells
// of it.
func (m Model) recordOnView() tea.Cmd {
	if _, t, ok := m.current(); ok && t.run == "" && t.change.Dir != "" {
		return m.loadRecord(t.change.Dir)
	}
	return nil
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

func (m Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p, t, has := m.current()
	if m.mode != reading {
		switch k.Type {
		case tea.KeyEsc:
			m.mode = reading
			m.input.Blur()
			m.input.SetValue("")
			return m, nil
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			mode := m.mode
			m.mode = reading
			m.input.Blur()
			m.input.SetValue("")
			if text == "" {
				return m, nil
			}
			if mode == intent {
				m.sel = 0 // the new task comes first
				return m, m.start(p.Path, text)
			}
			run := t.run
			return m, func() tea.Msg { return result(m.client.Refuse(m.ctx, run, text), "sent back") }
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd
	}
	s, live := m.client.Run(t.run)
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit // the runs go on in the service
	case "n":
		if len(m.projects) > 0 {
			m.mode, m.input.Prompt = intent, "intent for "+p.Name+" ▸ "
			return m, m.input.Focus()
		}
	case "a":
		if live && s.Question != "" {
			run := t.run
			return m, func() tea.Msg { return result(m.client.Accept(m.ctx, run), "accepted") }
		}
	case "r":
		if live && s.Question != "" {
			m.mode, m.input.Prompt = ground, "why ▸ "
			return m, m.input.Focus()
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
		if has && t.change.Dir != "" {
			return m, open(filepath.Join(t.change.Dir, "change.md"))
		}
	case "down", "j":
		m.sel, m.unit = min(m.sel+1, max(0, len(m.tasks(p))-1)), 0
		return m, m.recordOnView()
	case "up", "k":
		m.sel, m.unit = max(m.sel-1, 0), 0
		return m, m.recordOnView()
	case "right", "l", "tab":
		if len(m.projects) > 0 {
			m.proj, m.sel, m.unit = (m.proj+1)%len(m.projects), 0, 0
			return m, m.recordOnView()
		}
	case "left", "h", "shift+tab":
		if len(m.projects) > 0 {
			m.proj, m.sel, m.unit = (m.proj+len(m.projects)-1)%len(m.projects), 0, 0
			return m, m.recordOnView()
		}
	case "]":
		m.unit = min(m.unit+1, max(0, len(s.Agent)-1))
	case "[":
		m.unit = max(m.unit-1, 0)
	}
	return m, nil
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
	return noteMsg(done)
}

// open shows a file in the person's editor, or the system's viewer.
func open(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	cmd := exec.Command("open", path)
	if editor != "" {
		cmd = exec.Command(editor, path)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return noteMsg(path + " could not be opened: " + err.Error())
		}
		return nil
	})
}

var (
	faint   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8a8a85", Dark: "#7a7a75"})
	strong  = lipgloss.NewStyle().Bold(true)
	good    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#3b6d11", Dark: "#97c459"})
	bad     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#a32d2d", Dark: "#f09595"})
	active  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#185fa5", Dark: "#85b7eb"})
	waiting = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#854f0b", Dark: "#fac775"})
	rule    = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#d3d1c7", Dark: "#444441"})
)

// View draws the screen: the projects and their tasks on the left, the
// task on view on the right, and the line to type on at the foot.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	top := strong.Render("iflow") + faint.Render(" · "+m.term.name)
	body := max(4, m.height-3)
	side := min(40, max(24, m.width/3))
	main := m.width - side - 3
	return top + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(side).Height(body).MaxHeight(body).Render(m.sidebar(side, body)),
		faint.Render(" │ "),
		lipgloss.NewStyle().Width(main).Height(body).MaxHeight(body).Render(m.task(main, body))) + "\n" + m.foot()
}

// sidebar is every project, the one on view open on its tasks.
func (m Model) sidebar(width, height int) string {
	if len(m.projects) == 0 {
		return faint.Render("no projects yet")
	}
	var lines []string
	for i, p := range m.projects {
		ts := m.tasks(p)
		if i != m.proj {
			lines = append(lines, faint.Render(ansi.Truncate(fmt.Sprintf("▸ %s  %d", p.Name, len(ts)), width, "…")))
			continue
		}
		lines = append(lines, strong.Render(ansi.Truncate("▾ "+p.Name, width, "…")))
		room := max(1, height-len(m.projects))
		first := max(0, min(m.sel-room/2, len(ts)-room))
		for j, t := range ts[first:min(len(ts), first+room)] {
			what := oneLine(t.change.Statement)
			if what == "" {
				what = t.change.Name
			}
			line := ansi.Truncate(m.taskMark(t)+" "+what, width-2, "…")
			if first+j == m.sel {
				line = strong.Render("› ") + line
			} else {
				line = "  " + line
			}
			lines = append(lines, line)
		}
		if len(ts) == 0 {
			lines = append(lines, faint.Render("  no task yet — n"))
		}
	}
	return strings.Join(lines, "\n")
}

// taskMark is how a task stands, in one sign: by its run where one carries
// it, by its folder where not.
func (m Model) taskMark(t task) string {
	if s, ok := m.client.Run(t.run); ok && !s.Finished {
		return mark(s)
	}
	switch t.change.State {
	case protocol.Landed:
		return good.Render("✓")
	case protocol.Carried:
		return active.Render("●")
	case protocol.Closed:
		return faint.Render("–")
	}
	return bad.Render("✗")
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// task is the task on view: as its run tells it, or as its folder keeps it.
func (m Model) task(width, height int) string {
	_, t, ok := m.current()
	if !ok {
		return "\n" + faint.Render("  no task here yet — press n and say what you want")
	}
	if s, live := m.client.Run(t.run); live {
		out := m.head(s, width)
		h := height - 3
		if s.Question != "" {
			h -= 3
		}
		out += m.columns(s, width, max(3, h))
		if s.Question != "" {
			out += m.question(s, width)
		}
		return out
	}
	return m.history(t.change, width, height)
}

// history is a change no run carries now: how far it went, how it stands,
// and its record.
func (m Model) history(c protocol.Change, width, height int) string {
	var areas []string
	for n := 1; n <= 6; n++ {
		name := fmt.Sprintf("%d %s", n, areaNames[n])
		switch {
		case n <= c.Reached:
			areas = append(areas, good.Render(name+" ✓"))
		case n == c.Reached+1 && c.State == protocol.Stopped:
			areas = append(areas, bad.Render(name+" ✗"))
		default:
			areas = append(areas, faint.Render(name))
		}
	}
	title := m.term.link(filepath.Join(c.Dir, "change.md"), ansi.Truncate(oneLine(c.Statement), width-2, "…"))
	out := rule.Width(width).Render(title) + "\n" + strings.Join(areas, "  ") + "   " + faint.Render(string(c.State)) + "\n"
	rec := m.records[c.Dir]
	lines := []string{faint.Render("record")}
	for _, x := range rec[max(0, len(rec)-(height-7)):] {
		lines = append(lines, ansi.Truncate(fmt.Sprintf("%2d %s %-13s %s", x.N, x.At, x.Stage, oneLine(x.Note)), width, "…"))
	}
	out += strings.Join(lines, "\n") + "\n\n"
	if c.State == protocol.Stopped {
		return out + faint.Render("[R] resume from where it stopped   [o] open change.md")
	}
	return out + faint.Render("[o] open change.md")
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

// head is the run's task, the six areas as they stand, and what it spent.
func (m Model) head(s service.State, width int) string {
	var areas []string
	for n := 1; n <= 6; n++ {
		name := fmt.Sprintf("%d %s", n, areaNames[n])
		switch s.Areas[n] {
		case protocol.Done:
			areas = append(areas, good.Render(name+" ✓"))
		case protocol.Working:
			areas = append(areas, active.Render(name+" ●"))
		case protocol.Waiting:
			areas = append(areas, waiting.Render(name+" ?"))
		case protocol.Failed:
			areas = append(areas, bad.Render(name+" ✗"))
		default:
			areas = append(areas, faint.Render(name))
		}
	}
	spent := faint.Render(fmt.Sprintf("$%.3f · %dk in", s.Spent.CostUSD, (s.Spent.Input+s.Spent.CacheRead+s.Spent.CacheWrite)/1000))
	task := ansi.Truncate(oneLine(s.Statement), width-2, "…")
	if s.Dir != "" {
		task = m.term.link(filepath.Join(s.Dir, "change.md"), task)
	}
	return rule.Width(width).Render(task) + "\n" + strings.Join(areas, "  ") + "   " + spent + "\n"
}

// units is the run's units with an agent at work, in order.
func units(s service.State) []string {
	out := make([]string, 0, len(s.Agent))
	for u := range s.Agent {
		out = append(out, u)
	}
	slices.Sort(out)
	return out
}

// columns is the path so far beside the agent of the unit on view.
func (m Model) columns(s service.State, width, height int) string {
	left := width * 2 / 5
	right := width - left - 3
	var l []string
	us := units(s)
	if len(us) > 0 {
		l = append(l, faint.Render("units"))
		for i, u := range us {
			line := unitMark(s.Agent[u]) + " " + u
			if i == m.unit {
				line = strong.Render("› " + line)
			} else {
				line = "  " + line
			}
			l = append(l, line)
		}
		l = append(l, "")
	}
	l = append(l, faint.Render("path"))
	steps := s.Steps[max(0, len(s.Steps)-(height-len(l))):]
	for _, st := range steps {
		text := fmt.Sprintf("%d %s", st.Area, st.Text)
		if st.Bad {
			text = bad.Render(text)
		}
		l = append(l, ansi.Truncate(text, left, "…"))
	}
	var r []string
	if len(us) > 0 {
		u := us[min(m.unit, len(us)-1)]
		r = append(r, faint.Render(u+" · agent"))
		events := s.Agent[u]
		for _, e := range events[max(0, len(events)-(height-1)):] {
			r = append(r, ansi.Truncate(event(e), right, "…"))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(left).Height(height).Render(strings.Join(l, "\n")),
		faint.Render(" │ "),
		lipgloss.NewStyle().Width(right).Height(height).Render(strings.Join(r, "\n"))) + "\n"
}

// unitMark is how a unit stands by the last thing its agent did.
func unitMark(events []protocol.Event) string {
	if len(events) == 0 {
		return " "
	}
	last := events[len(events)-1]
	switch {
	case last.Kind == "ended" && strings.HasPrefix(last.Detail, "candidate"):
		return good.Render("✓")
	case last.Kind == "ended":
		return bad.Render("✗")
	}
	return active.Render("●")
}

// event is one thing an agent did, in a line.
func event(e protocol.Event) string {
	detail := strings.Join(strings.Fields(e.Detail), " ")
	switch e.Kind {
	case "called":
		return "› " + strings.ToLower(e.Tool) + " " + detail
	case "got":
		return faint.Render("  " + detail)
	case "refused":
		return bad.Render("  refused: " + detail)
	case "ended":
		return strong.Render("■ " + detail)
	}
	return faint.Render(e.Kind + " " + detail)
}

// question is what the run waits on the person for, and how to answer.
func (m Model) question(s service.State, width int) string {
	box := lipgloss.NewStyle().Width(width).BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#ba7517", Dark: "#ef9f27"})
	return box.Render(waiting.Render("waiting for you · ")+ansi.Truncate(s.Question, width-20, "…")) + "\n" +
		faint.Render("[a] accept   [r] send back with why   [o] open change.md") + "\n"
}

// foot is the line the person types on, or what the keys do.
func (m Model) foot() string {
	if m.mode != reading {
		return m.input.View()
	}
	keys := faint.Render("↑↓ task · ←→ project · [ ] unit · n new · a accept · r send back · s stop · q leave (runs go on)")
	if m.note != "" {
		keys = m.note + faint.Render("  ·  ") + keys
	}
	return ansi.Truncate(keys, m.width, "…")
}
