package mcp

import (
	"context"
	"errors"
	"fmt"

	"oriva/backend-go/db/postgres"
	"oriva/backend-go/models/interview"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SchemaVersion is the version of the tool surface (docs/ARCHITECTURE.md §2).
const SchemaVersion = "oriva.tools.v1"

// --- consumed contracts (consumer-defined; keep mcp testable without a DB) ---

type interviewReader interface {
	OrgOf(ctx context.Context, sessionID string) (string, error)
	PlanData(ctx context.Context, orgID, sessionID string) (*postgres.PlanData, error)
}

type responseRecorder interface {
	Record(ctx context.Context, sessionID string, turnIndex int, question, answer string) (int, error)
}

type stateAdvancer interface {
	Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)
}

// --- get_interview_plan ---

type PlanIn struct {
	SessionID string `json:"session_id" jsonschema:"the interview session id"`
}

type PlanOut struct {
	SessionID       string   `json:"session_id"`
	State           string   `json:"state"`
	JobTitle        string   `json:"job_title"`
	JobDescription  string   `json:"job_description"`
	CandidateName   string   `json:"candidate_name"`
	CandidateResume string   `json:"candidate_resume"`
	Questions       []string `json:"questions"`
	SchemaVersion   string   `json:"schema_version"`
}

var cannedQuestions = []string{
	"Walk me through a recent project you're proud of and your specific role in it.",
	"Tell me about a technical decision you made that you'd revisit today.",
	"Describe a time you disagreed with a teammate on an approach. How did it resolve?",
}

func (h *handlers) getInterviewPlan(
	ctx context.Context, _ *mcpsdk.CallToolRequest, in PlanIn,
) (*mcpsdk.CallToolResult, PlanOut, error) {
	org, err := h.orgOf(ctx, in.SessionID)
	if err != nil {
		return toolErr(err)
	}
	p, err := h.interviews.PlanData(ctx, org, in.SessionID)
	if err != nil {
		return toolErr(err)
	}
	return nil, PlanOut{
		SessionID:       in.SessionID,
		State:           p.State,
		JobTitle:        p.JobTitle,
		JobDescription:  p.JobDescription,
		CandidateName:   p.CandidateName,
		CandidateResume: p.CandidateResume,
		Questions:       cannedQuestions,
		SchemaVersion:   SchemaVersion,
	}, nil
}

// --- retrieve_context (stub) ---

type RetrieveIn struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query" jsonschema:"what to search the resume/JD for"`
	K         int    `json:"k,omitempty" jsonschema:"max chunks to return (default 5)"`
}

type RetrieveOut struct {
	Chunks []string `json:"chunks"`
	Note   string   `json:"note"`
}

func (h *handlers) retrieveContext(
	ctx context.Context, _ *mcpsdk.CallToolRequest, in RetrieveIn,
) (*mcpsdk.CallToolResult, RetrieveOut, error) {
	if _, err := h.orgOf(ctx, in.SessionID); err != nil {
		return toolErrRetrieve(err)
	}
	return nil, RetrieveOut{
		Chunks: []string{},
		Note:   "pgvector retrieval is not implemented in this slice",
	}, nil
}

// --- record_turn ---

type RecordIn struct {
	SessionID string `json:"session_id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	TurnIndex int    `json:"turn_index,omitempty" jsonschema:"omit to auto-append after the last turn"`
}

type RecordOut struct {
	Recorded  bool `json:"recorded"`
	TurnIndex int  `json:"turn_index"`
}

func (h *handlers) recordTurn(
	ctx context.Context, _ *mcpsdk.CallToolRequest, in RecordIn,
) (*mcpsdk.CallToolResult, RecordOut, error) {
	if _, err := h.orgOf(ctx, in.SessionID); err != nil {
		return toolErrRecord(err)
	}
	idx, err := h.responses.Record(ctx, in.SessionID, in.TurnIndex, in.Question, in.Answer)
	if errors.Is(err, postgres.ErrConflict) {
		return nil, RecordOut{}, fmt.Errorf("turn_index %d already recorded", in.TurnIndex)
	}
	if err != nil {
		return nil, RecordOut{}, err
	}
	return nil, RecordOut{Recorded: true, TurnIndex: idx}, nil
}

// --- advance_state ---

type AdvanceIn struct {
	SessionID string `json:"session_id"`
	ToState   string `json:"to_state" jsonschema:"target session state (see get_interview_plan / session-states)"`
	Reason    string `json:"reason,omitempty"`
}

type AdvanceOut struct {
	State      string `json:"state"`
	StateLabel string `json:"state_label"`
}

func (h *handlers) advanceState(
	ctx context.Context, _ *mcpsdk.CallToolRequest, in AdvanceIn,
) (*mcpsdk.CallToolResult, AdvanceOut, error) {
	org, err := h.orgOf(ctx, in.SessionID)
	if err != nil {
		return toolErrAdvance(err)
	}
	d, err := h.sessions.Advance(ctx, org, in.SessionID, in.ToState, in.Reason, "ai-service")
	if err != nil {
		return nil, AdvanceOut{}, err // *apxerrors.Error message reaches the LLM
	}
	return nil, AdvanceOut{State: d.State, StateLabel: d.StateLabel}, nil
}

// --- helpers ---

type handlers struct {
	interviews interviewReader
	responses  responseRecorder
	sessions   stateAdvancer
}

func (h *handlers) orgOf(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		return "", errors.New("session_id is required")
	}
	org, err := h.interviews.OrgOf(ctx, sessionID)
	if errors.Is(err, postgres.ErrNotFound) {
		return "", errors.New("session not found")
	}
	return org, err
}

func toolErr(err error) (*mcpsdk.CallToolResult, PlanOut, error) {
	return nil, PlanOut{}, err
}
func toolErrRetrieve(err error) (*mcpsdk.CallToolResult, RetrieveOut, error) {
	return nil, RetrieveOut{}, err
}
func toolErrRecord(err error) (*mcpsdk.CallToolResult, RecordOut, error) {
	return nil, RecordOut{}, err
}
func toolErrAdvance(err error) (*mcpsdk.CallToolResult, AdvanceOut, error) {
	return nil, AdvanceOut{}, err
}
