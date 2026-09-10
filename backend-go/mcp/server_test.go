package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"oriva/backend-go/db/postgres"
	"oriva/backend-go/models/interview"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeInterviews struct {
	org     string
	orgErr  error
	plan    *postgres.PlanData
	planErr error
}

func (f fakeInterviews) OrgOf(context.Context, string) (string, error) { return f.org, f.orgErr }
func (f fakeInterviews) PlanData(context.Context, string, string) (*postgres.PlanData, error) {
	return f.plan, f.planErr
}

type fakeResponses struct {
	gotQ, gotA string
	idx        int
	err        error
}

func (f *fakeResponses) Record(_ context.Context, _ string, turnIndex int, q, a string) (int, error) {
	f.gotQ, f.gotA = q, a
	if f.err != nil {
		return 0, f.err
	}
	if turnIndex > 0 {
		return turnIndex, nil
	}
	return f.idx, nil
}

type fakeSessions struct {
	gotTo, gotReason, gotActor string
	detail                     *interview.Detail
	err                        error
}

func (f *fakeSessions) Advance(_ context.Context, _, _, to, reason, actor string) (*interview.Detail, error) {
	f.gotTo, f.gotReason, f.gotActor = to, reason, actor
	if f.err != nil {
		return nil, f.err
	}
	return f.detail, nil
}

func connect(t *testing.T, d Deps) *mcpsdk.ClientSession {
	t.Helper()
	srv := NewServer(d)
	st, ct := mcpsdk.NewInMemoryTransports()
	_, err := srv.MCP().Connect(context.Background(), st, nil)
	require.NoError(t, err)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcpsdk.ClientSession, name string, args any, out any) *mcpsdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	if out != nil && !res.IsError {
		require.NoError(t, json.Unmarshal(mustJSON(t, res.StructuredContent), out))
	}
	return res
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestListTools(t *testing.T) {
	cs := connect(t, Deps{Interviews: fakeInterviews{}, Responses: &fakeResponses{}, Sessions: &fakeSessions{}})
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		assert.NotNil(t, tool.InputSchema)
	}
	for _, want := range []string{"get_interview_plan", "retrieve_context", "record_turn", "advance_state"} {
		assert.True(t, names[want], "missing tool %s", want)
	}
	assert.Len(t, res.Tools, 4)
}

func TestAdvanceStateTool(t *testing.T) {
	fs := &fakeSessions{detail: &interview.Detail{State: "invited", StateLabel: "Invited"}}
	cs := connect(t, Deps{Interviews: fakeInterviews{org: "o1"}, Responses: &fakeResponses{}, Sessions: fs})

	var out AdvanceOut
	res := call(t, cs, "advance_state", map[string]any{
		"session_id": "s1", "to_state": "invited", "reason": "emailed",
	}, &out)
	assert.False(t, res.IsError)
	assert.Equal(t, "invited", out.State)
	assert.Equal(t, "Invited", out.StateLabel)
	assert.Equal(t, "invited", fs.gotTo)
	assert.Equal(t, "emailed", fs.gotReason)
	assert.Equal(t, "ai-service", fs.gotActor)
}

func TestRecordTurnTool(t *testing.T) {
	fr := &fakeResponses{idx: 3}
	cs := connect(t, Deps{Interviews: fakeInterviews{org: "o1"}, Responses: fr, Sessions: &fakeSessions{}})

	var out RecordOut
	call(t, cs, "record_turn", map[string]any{
		"session_id": "s1", "question": "why go?", "answer": "concurrency",
	}, &out)
	assert.True(t, out.Recorded)
	assert.Equal(t, 3, out.TurnIndex)
	assert.Equal(t, "why go?", fr.gotQ)
	assert.Equal(t, "concurrency", fr.gotA)
}

func TestGetInterviewPlanTool(t *testing.T) {
	plan := &postgres.PlanData{
		State: "scheduled", JobTitle: "Backend Eng", CandidateName: "Alice", CandidateResume: "10y",
	}
	cs := connect(t, Deps{Interviews: fakeInterviews{org: "o1", plan: plan}, Responses: &fakeResponses{}, Sessions: &fakeSessions{}})

	var out PlanOut
	call(t, cs, "get_interview_plan", map[string]any{"session_id": "s1"}, &out)
	assert.Equal(t, "Backend Eng", out.JobTitle)
	assert.Equal(t, "Alice", out.CandidateName)
	assert.Equal(t, SchemaVersion, out.SchemaVersion)
	assert.Len(t, out.Questions, 3)
}

func TestRetrieveContextStub(t *testing.T) {
	cs := connect(t, Deps{Interviews: fakeInterviews{org: "o1"}, Responses: &fakeResponses{}, Sessions: &fakeSessions{}})
	var out RetrieveOut
	call(t, cs, "retrieve_context", map[string]any{"session_id": "s1", "query": "kafka"}, &out)
	assert.Empty(t, out.Chunks)
	assert.NotEmpty(t, out.Note)
}

func TestUnknownSessionIsToolError(t *testing.T) {
	cs := connect(t, Deps{
		Interviews: fakeInterviews{orgErr: postgres.ErrNotFound},
		Responses:  &fakeResponses{}, Sessions: &fakeSessions{},
	})
	res := call(t, cs, "advance_state", map[string]any{"session_id": "nope", "to_state": "invited"}, nil)
	assert.True(t, res.IsError)
	require.NotEmpty(t, res.Content)
	if tc, ok := res.Content[0].(*mcpsdk.TextContent); ok {
		assert.Contains(t, tc.Text, "session not found")
	}
}
