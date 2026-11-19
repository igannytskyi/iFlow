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
)

// Version is the version of the protocol this side speaks.
const Version = 1

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
)

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
