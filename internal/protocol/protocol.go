// Package protocol is how a screen speaks to the service that carries
// runs: JSON-RPC 2.0, one message a line, over a Unix socket. Every message
// has a type here; nothing crosses as text to be read for meaning.
//
// A screen calls methods — to start, answer, stop and watch runs — and the
// service sends it, unasked, what changed in the runs it watches. Version
// is raised whenever a message changes in a way an older side would
// misread; the two sides compare it first, at initialize.
package protocol

import (
	"encoding/json"
	"time"

	"github.com/igannytskyi/iflow/internal/meta"
)

// Version is the version of the protocol this side speaks.
const Version = 4

// The methods a screen calls.
const (
	Initialize = "initialize" // InitializeParams → InitializeResult
	Runs       = "runs"       // nothing → []RunSummary
	Start      = "run.start"  // StartParams → RunRef
	Resume     = "run.resume" // ResumeParams → RunRef
	Reopen     = "run.reopen" // ReopenParams → RunRef
	Answer     = "run.answer" // AnswerParams → nothing
	Stop       = "run.stop"   // RunRef → nothing
	Watch      = "run.watch"  // RunRef → nothing; Changed follows
	Unwatch    = "run.unwatch"
	Projects   = "projects"      // ProjectsParams → []Project
	Record     = "change.record" // ChangeRef → []Crossing
	Get        = "change.get"    // ChangeRef → ChangeDoc
	Trace      = "unit.trace"    // TraceParams → []Event
	Ask        = "chat.ask"      // AskParams → nothing; ChatChanged follows for who watches
	Listen     = "chat.watch"    // ChatRef → nothing; ChatChanged follows
)

// ChatChanged is what the service sends a screen watching a conversation,
// whenever it moves: a ChatState.
const ChatChanged = "chat.changed"

// Changed is what the service sends a screen watching a run, whenever the
// run moves: a RunDelta.
const Changed = "run.changed"

// InitializeParams is who calls, and in which version.
type InitializeParams struct {
	Version int    `json:"version"`
	Client  string `json:"client"`
}

// InitializeResult is the service's version, and who it is.
type InitializeResult struct {
	Version int    `json:"version"`
	Server  string `json:"server"`
}

// StartParams starts a task on a project: the estate's folder, the
// sentence, and the escalations it is meant to close.
type StartParams struct {
	Project   string   `json:"project"`
	Statement string   `json:"statement"`
	Settles   []string `json:"settles,omitempty"`
}

// ResumeParams carries a stopped change on from where it stopped.
type ResumeParams struct {
	Dir     string   `json:"dir"`
	Settles []string `json:"settles,omitempty"`
}

// ReopenParams carries a change again from its specification (1) or its
// plan (2), with what the person says is to change.
type ReopenParams struct {
	Dir     string   `json:"dir"`
	From    int      `json:"from"`
	Notes   string   `json:"notes"`
	Settles []string `json:"settles,omitempty"`
}

// AnswerParams answers the question a run waits on: accepted, or refused
// with why.
type AnswerParams struct {
	Run    string `json:"run"`
	Accept bool   `json:"accept"`
	Why    string `json:"why,omitempty"`
}

// RunRef names a run the service carries.
type RunRef struct {
	Run string `json:"run"`
}

// RunSummary is a run in a line.
type RunSummary struct {
	Run       string `json:"run"`
	Project   string `json:"project"`
	Statement string `json:"statement"`
	Dir       string `json:"dir,omitempty"`
	Question  string `json:"question,omitempty"`
	Finished  bool   `json:"finished"`
	Error     string `json:"error,omitempty"`
}

// Status is how an area of the path stands.
type Status string

// The states an area passes through, as the flow names them.
const (
	Idle    Status = "idle"
	Working Status = "working"
	Waiting Status = "waiting for you"
	Done    Status = "done"
	Failed  Status = "stopped"
)

// Step is one thing that happened in a run.
type Step struct {
	At     time.Time `json:"at"`
	Area   int       `json:"area"`
	Text   string    `json:"text"`
	Object string    `json:"object,omitempty"`
	Bad    bool      `json:"bad,omitempty"`
}

// Event is one thing an agent did.
type Event struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Tool   string    `json:"tool,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// Usage is what was spent.
type Usage struct {
	Input      int     `json:"input"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Output     int     `json:"output"`
	CostUSD    float64 `json:"cost_usd"`
}

// RunDelta is what moved in a run since the screen was last told: the
// steps from StepsFrom on, each unit's agent events from AgentFrom on, and
// the rest as it stands now. A screen holds what it was told and appends.
type RunDelta struct {
	Run       string             `json:"run"`
	Project   string             `json:"project"`
	Statement string             `json:"statement"`
	Slug      string             `json:"slug,omitempty"`
	Dir       string             `json:"dir,omitempty"`
	Areas     [7]Status          `json:"areas"`
	StepsFrom int                `json:"steps_from"`
	Steps     []Step             `json:"steps,omitempty"`
	AgentFrom map[string]int     `json:"agent_from,omitempty"`
	Agent     map[string][]Event `json:"agent,omitempty"`
	Question  string             `json:"question,omitempty"`
	Reviewed  string             `json:"reviewed,omitempty"`
	Spent     Usage              `json:"spent"`
	Finished  bool               `json:"finished"`
	Error     string             `json:"error,omitempty"`
}

// Message is one JSON-RPC 2.0 message: a call (Method and ID), a
// notification (Method alone), or an answer (ID, and Result or Error).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a call that could not be answered.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// The codes of JSON-RPC 2.0, and the service's own.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	// Refused is a call understood and refused: a version not spoken, a run
	// not known, a question not asked.
	Refused = -32000
)

// Project is a project the person works on, and its changes, newest first.
type Project struct {
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Changes []Change `json:"changes"`
}

// Change is a change carried on a project, as its folder says it stands.
type Change struct {
	Dir       string `json:"dir"`
	Name      string `json:"name"`
	Statement string `json:"statement"`
	// Reached is the last area that wrote into it, 0 to 6.
	Reached  int         `json:"reached"`
	State    ChangeState `json:"state"`
	Modified time.Time   `json:"modified"`
}

// ChangeState is how a change stands.
type ChangeState string

// The ways a change can stand.
const (
	Carried ChangeState = "working" // a run carries it now, here or elsewhere
	Landed  ChangeState = "landed"
	Closed  ChangeState = "closed"
	Stopped ChangeState = "stopped"
)

// ChangeRef names a change by its folder.
type ChangeRef struct {
	Dir string `json:"dir"`
}

// Crossing is one line of a change's record: what crossed, when, and why.
type Crossing struct {
	N      int    `json:"n"`
	At     string `json:"at"`
	Stage  string `json:"stage"`
	Object string `json:"object"`
	Note   string `json:"note,omitempty"`
}

// ProjectsParams asks for the projects the person works on, and for those
// in Also besides — a project a screen was opened on.
type ProjectsParams struct {
	Also []string `json:"also,omitempty"`
}

// ChangeDoc is a change as its folder holds it: the change described, every
// document of the path it has reached, in the data model, and what its work
// cost.
type ChangeDoc struct {
	Change        Change              `json:"change"`
	Document      string              `json:"document,omitempty"` // change.md
	Specification *meta.Specification `json:"specification,omitempty"`
	Plan          *meta.Plan          `json:"plan,omitempty"`
	Admission     *meta.Admission     `json:"admission,omitempty"`
	Execution     *meta.Execution     `json:"execution,omitempty"`
	Assurance     *meta.Assurance     `json:"assurance,omitempty"`
	Landing       *meta.Landing       `json:"landing,omitempty"`
	Record        []Crossing          `json:"record,omitempty"`
	Costs         []Cost              `json:"costs,omitempty"`
}

// Cost is one piece of model or tool work a change did, and what it spent.
type Cost struct {
	Area       int     `json:"area"`
	Unit       string  `json:"unit,omitempty"`
	What       string  `json:"what"`
	Actor      string  `json:"actor"`
	Model      string  `json:"model,omitempty"`
	Handed     int     `json:"handed,omitempty"`
	Input      int     `json:"input"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Output     int     `json:"output"`
	CostUSD    float64 `json:"cost_usd"`
	ElapsedMS  int64   `json:"elapsed_ms"`
}

// TraceParams names a candidate of a change, whose agent's steps are asked
// for.
type TraceParams struct {
	Dir       string `json:"dir"`
	Candidate string `json:"candidate"`
}

// ChatRef names a conversation: about a task, by the task's folder, or about
// a project, by the project's folder and no task.
type ChatRef struct {
	Project string `json:"project"`
	Dir     string `json:"dir,omitempty"`
}

// AskParams puts the person's words to the model, in a conversation.
type AskParams struct {
	ChatRef
	Text string `json:"text"`
}

// ChatMessage is one turn of a conversation.
type ChatMessage struct {
	At      time.Time `json:"at"`
	Role    string    `json:"role"` // person or model
	Text    string    `json:"text"`
	CostUSD float64   `json:"cost_usd,omitempty"`
	Err     bool      `json:"err,omitempty"`
	// Sources are the pages of the web the answer cites.
	Sources []Source `json:"sources,omitempty"`
}

// Source is a page of the web an answer cites.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// ChatState is a conversation as it stands: its turns, the answer being
// written, and what it spent.
type ChatState struct {
	ChatRef
	Messages []ChatMessage `json:"messages"`
	Partial  string        `json:"partial,omitempty"`
	Waiting  bool          `json:"waiting"`
	CostUSD  float64       `json:"cost_usd"`
}
