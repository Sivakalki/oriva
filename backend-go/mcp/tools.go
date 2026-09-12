package mcp

import (
	"context"
	"errors"
	"fmt"

	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/interview_repo"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SchemaVersion is the version of the tool surface (docs/ARCHITECTURE.md §2).
const SchemaVersion = "oriva.tools.v1"

// --- consumed contracts (consumer-defined; keep mcp testable without a DB) ---

type interviewReader interface {
	OrgOf(ctx context.Context, sessionID string) (string, error)
	PlanData(ctx context.Context, orgID, sessionID string) (*interview_repo.PlanData, error)
}

type responseRecorder interface {
	Record(ctx context.Context, sessionID string, turnIndex int, question, answer string) (int, error)
}

type stateAdvancer interface {
	Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)
}

// turnScorer scores a just-recorded turn in the background (services/scoring).
type turnScorer interface {
	ScoreTurn(ctx context.Context, orgID, sessionID string, turnIndex int, question, answer string)
}

// --- get_interview_plan ---

type PlanIn struct {
	SessionID string `json:"session_id" jsonschema:"the interview session id"`
}

type PlanOut struct {
	SessionID       string `json:"session_id"`
	State           string `json:"state"`
	JobTitle        string `json:"job_title"`
	JobDescription  string `json:"job_description"`
	CandidateName   string `json:"candidate_name"`
	CandidateResume string `json:"candidate_resume"`
	DurationMinutes int    `json:"duration_minutes"`
	SchemaVersion   string `json:"schema_version"`
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
	// No canned questions: the LLM generates every question itself, grounded
	// in job_description and candidate_resume (see ai-service's system
	// prompt, pipelines/context.py) -- that's what makes them dynamic.
	return nil, PlanOut{
		SessionID:       in.SessionID,
		State:           p.State,
		JobTitle:        p.JobTitle,
		JobDescription:  p.JobDescription,
		CandidateName:   p.CandidateName,
		CandidateResume: p.CandidateResume,
		DurationMinutes: p.DurationMinutes,
		SchemaVersion:   SchemaVersion,
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

	if h.scoring != nil {
		org, orgErr := h.orgOf(ctx, in.SessionID)
		if orgErr == nil {
			// Detached from ctx (which dies when this tool call returns) and
			// backgrounded: scoring never adds latency to the live turn.
			go h.scoring.ScoreTurn(context.Background(), org, in.SessionID, idx, in.Question, in.Answer)
		}
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
	scoring    turnScorer // nil-safe: record_turn just skips the scoring hook
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
func toolErrRecord(err error) (*mcpsdk.CallToolResult, RecordOut, error) {
	return nil, RecordOut{}, err
}
func toolErrAdvance(err error) (*mcpsdk.CallToolResult, AdvanceOut, error) {
	return nil, AdvanceOut{}, err
}
