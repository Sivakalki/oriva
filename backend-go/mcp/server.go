// Package mcp implements the Go↔Python tool boundary: an MCP server over
// Streamable HTTP owning the oriva.tools.v1 tool schemas (docs/ARCHITECTURE.md §2).
package mcp

import (
	"net/http"

	"oriva/backend-go/utils/buildinfo"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// Deps are the collaborators the MCP tools need.
type Deps struct {
	Interviews interviewReader
	Responses  responseRecorder
	Sessions   stateAdvancer
	Scoring    turnScorer // optional: nil disables per-turn scoring
	Logger     *zap.Logger
}

// Server wraps a configured *mcpsdk.Server.
type Server struct {
	mcp    *mcpsdk.Server
	logger *zap.Logger
}

// NewServer builds the MCP server and registers the four tools.
func NewServer(d Deps) *Server {
	impl := &mcpsdk.Implementation{Name: "oriva-backend", Version: buildinfo.Version}
	srv := mcpsdk.NewServer(impl, &mcpsdk.ServerOptions{
		Instructions: "Interview orchestration tools, schema version " + SchemaVersion +
			". Every tool takes a session_id; the session determines the organization.",
	})

	h := &handlers{interviews: d.Interviews, responses: d.Responses, sessions: d.Sessions, scoring: d.Scoring}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_interview_plan",
		Description: "Return the job, resume, current state and opening questions for a session.",
	}, h.getInterviewPlan)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "retrieve_context",
		Description: "Retrieve resume/JD passages relevant to a query (not yet implemented).",
	}, h.retrieveContext)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "record_turn",
		Description: "Persist one question/answer turn of the interview transcript.",
	}, h.recordTurn)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "advance_state",
		Description: "Move the interview session to a new state. Rejected if the transition is illegal.",
	}, h.advanceState)

	return &Server{mcp: srv, logger: d.Logger}
}

// Handler returns the Streamable HTTP handler to mount at the MCP path.
func (s *Server) Handler() http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return s.mcp },
		nil,
	)
}

// MCP returns the underlying SDK server (used by tests to connect in-memory).
func (s *Server) MCP() *mcpsdk.Server { return s.mcp }
