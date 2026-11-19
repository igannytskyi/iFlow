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
	project string
	term    terminal

	width, height int

	runs  []string // the runs of this project, oldest first
	cur   string   // the run on view
	unit  int      // the unit whose agent is on view
	input textinput.Model
	mode  typing
	note  string // what the screen last has to say: an error, an answer

	// told is, per run, the last thing the person was notified of, so that
	// nothing is notified twice.
	told map[string]string
}

// New is the screen of project, speaking to the service through client;
// moved carries the ids of the runs that moved.
func New(ctx context.Context, client *service.Client, moved chan string, project string) Model {
	in := textinput.New()
	in.Prompt = "intent ▸ "
	in.CharLimit = 2000
	in.Cursor.SetMode(cursor.CursorStatic) // a cursor that does not blink asks nothing of the screen
	return Model{ctx: ctx, client: client, moved: moved, project: project, term: detect(os.Getenv),
		input: in, told: map[string]string{}}
}

type (
	movedMsg string
	runsMsg  []protocol.RunSummary
	noteMsg  string
	lostMsg  struct{}
)

// Init lists the runs the service carries on this project and watches them.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.listRuns, m.waitMoved, m.waitLost)
}

func (m Model) listRuns() tea.Msg {
	runs, err := m.client.Runs(m.ctx)
	if err != nil {
		return noteMsg("the service could not list its runs: " + err.Error())
	}
	var mine []protocol.RunSummary
	for _, r := range runs {
		if r.Project == m.project {
			if err := m.client.Watch(m.ctx, r.Run); err != nil {
				return noteMsg(err.Error())
			}
			mine = append(mine, r)
		}
	}
	return runsMsg(mine)
}

func (m Model) waitMoved() tea.Msg { return movedMsg(<-m.moved) }

func (m Model) waitLost() tea.Msg {
	<-m.client.Done()
	return lostMsg{}
}

// Update applies what happened: a key, a run that moved, a window resized.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(10, msg.Width-12)
	case runsMsg:
		for _, r := range msg {
			if !slices.Contains(m.runs, r.Run) {
				m.runs = append(m.runs, r.Run)
			}
		}
		if m.cur == "" && len(m.runs) > 0 {
			m.cur = m.runs[len(m.runs)-1]
		}
	case movedMsg:
		run := string(msg)
		if !slices.Contains(m.runs, run) {
			m.runs = append(m.runs, run)
		}
		if m.cur == "" {
			m.cur = run
		}
		return m, tea.Batch(m.waitMoved, m.notify(run))
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

func (m Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				return m, m.start(text)
			}
			run := m.cur
			return m, func() tea.Msg { return result(m.client.Refuse(m.ctx, run, text), "sent back") }
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd
	}
	s, _ := m.client.Run(m.cur)
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit // the runs go on in the service
	case "n":
		m.mode, m.input.Prompt = intent, "intent ▸ "
		return m, m.input.Focus()
	case "a":
		if s.Question != "" {
			run := m.cur
			return m, func() tea.Msg { return result(m.client.Accept(m.ctx, run), "accepted") }
		}
	case "r":
		if s.Question != "" {
			m.mode, m.input.Prompt = ground, "why ▸ "
			return m, m.input.Focus()
		}
	case "s":
		if m.cur != "" && !s.Finished {
			run := m.cur
			return m, func() tea.Msg { return result(m.client.Stop(m.ctx, run), "asked to stop") }
		}
	case "o":
		if s.Dir != "" {
			return m, open(filepath.Join(s.Dir, "change.md"))
		}
	case "tab":
		m.cur, m.unit = step(m.runs, m.cur, 1), 0
	case "shift+tab":
		m.cur, m.unit = step(m.runs, m.cur, -1), 0
	case "down", "j":
		m.unit = min(m.unit+1, max(0, len(s.Agent)-1))
	case "up", "k":
		m.unit = max(m.unit-1, 0)
	}
	return m, nil
}

func (m Model) start(statement string) tea.Cmd {
	return func() tea.Msg {
		run, err := m.client.Start(m.ctx, protocol.StartParams{Project: m.project, Statement: statement})
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

// step is the run d places from cur, round the list.
func step(runs []string, cur string, d int) string {
	if len(runs) == 0 {
		return cur
	}
	i := slices.Index(runs, cur)
	return runs[((i+d)%len(runs)+len(runs))%len(runs)]
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

// View draws the screen.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strong.Render("iflow") + faint.Render(" · "+filepath.Base(m.project)+" · "+m.term.name) + "\n")
	b.WriteString(m.tabs() + "\n")
	s, ok := m.client.Run(m.cur)
	body := m.height - 6
	switch {
	case !ok:
		b.WriteString("\n" + faint.Render("  no task yet — press n and say what you want") + "\n")
		body -= 2
	default:
		b.WriteString(m.head(s))
		body -= 3
		if s.Question != "" {
			body -= 4
		}
		b.WriteString(m.columns(s, max(4, body)))
		if s.Question != "" {
			b.WriteString(m.question(s))
		}
	}
	b.WriteString(m.foot())
	return b.String()
}

// tabs is every run of the project, the one on view marked.
func (m Model) tabs() string {
	if len(m.runs) == 0 {
		return ""
	}
	var parts []string
	for _, id := range m.runs {
		s, _ := m.client.Run(id)
		label := mark(s) + " " + ansi.Truncate(s.Statement, 28, "…")
		if id == m.cur {
			label = strong.Render(label)
		} else {
			label = faint.Render(label)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, faint.Render("  │  "))
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
func (m Model) head(s service.State) string {
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
	task := ansi.Truncate(s.Statement, m.width-2, "…")
	if s.Dir != "" {
		task = m.term.link(filepath.Join(s.Dir, "change.md"), task)
	}
	return rule.Width(m.width).Render(task) + "\n" + strings.Join(areas, "  ") + "   " + spent + "\n"
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
func (m Model) columns(s service.State, height int) string {
	left := m.width * 2 / 5
	right := m.width - left - 3
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
func (m Model) question(s service.State) string {
	box := lipgloss.NewStyle().Width(m.width).BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#ba7517", Dark: "#ef9f27"})
	return box.Render(waiting.Render("waiting for you · ")+ansi.Truncate(s.Question, m.width-20, "…")) + "\n" +
		faint.Render("[a] accept   [r] send back with why   [o] open change.md") + "\n"
}

// foot is the line the person types on, or what the keys do.
func (m Model) foot() string {
	if m.mode != reading {
		return m.input.View()
	}
	keys := faint.Render("n new task · tab next · ↑↓ unit · s stop · q leave (runs go on)")
	if m.note != "" {
		keys = m.note + faint.Render("  ·  ") + keys
	}
	return ansi.Truncate(keys, m.width, "…")
}

// Connect speaks to the service at socket, starting one beside the screen
// where none listens: the runs it starts go on when the screen closes.
func Connect(ctx context.Context, socket string, moved chan string) (*service.Client, error) {
	changed := func(run string) {
		select {
		case moved <- run:
		default: // the screen will read every run again when it next draws
		}
	}
	if c, err := service.Dial(ctx, socket, "iflow tui", changed); err == nil {
		return c, nil
	}
	if err := spawn(socket); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := service.Dial(ctx, socket, "iflow tui", changed)
		if err == nil {
			return c, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the service did not come up at %s: %w", socket, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// spawn starts the service apart from the screen, writing what it says to a
// log beside its socket.
func spawn(socket string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(filepath.Dir(socket), "serve.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(self, "serve")
	cmd.Env = append(os.Environ(), "IFLOW_SOCKET="+socket)
	cmd.Stdout, cmd.Stderr = log, log
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
