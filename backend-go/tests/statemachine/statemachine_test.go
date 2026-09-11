package statemachine_test

import (
	"testing"

	"oriva/backend-go/statemachine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seededStates / seededTransitions mirror migration 0002 (docs/ARCHITECTURE.md §4).
func seededStates() []statemachine.State {
	return []statemachine.State{
		{Name: "scheduled", Label: "Scheduled", IsTerminal: false},
		{Name: "invited", Label: "Invited", IsTerminal: false},
		{Name: "ready", Label: "Ready", IsTerminal: false},
		{Name: "dispatched", Label: "Dispatched", IsTerminal: false},
		{Name: "in_progress", Label: "In progress", IsTerminal: false},
		{Name: "completed", Label: "Completed", IsTerminal: false},
		{Name: "scoring", Label: "Scoring", IsTerminal: false},
		{Name: "scored", Label: "Scored", IsTerminal: true},
		{Name: "declined", Label: "Declined", IsTerminal: true},
		{Name: "abandoned", Label: "Abandoned", IsTerminal: true},
		{Name: "interrupted", Label: "Interrupted", IsTerminal: false},
		{Name: "failed", Label: "Failed", IsTerminal: true},
	}
}

func seededTransitions() []statemachine.Transition {
	return []statemachine.Transition{
		{From: "scheduled", To: "invited"}, {From: "invited", To: "ready"}, {From: "ready", To: "dispatched"},
		{From: "dispatched", To: "in_progress"}, {From: "in_progress", To: "completed"},
		{From: "completed", To: "scoring"}, {From: "scoring", To: "scored"},
		{From: "ready", To: "declined"},
		{From: "invited", To: "abandoned"}, {From: "ready", To: "abandoned"},
		{From: "dispatched", To: "interrupted"}, {From: "in_progress", To: "interrupted"},
		{From: "interrupted", To: "dispatched"},
		{From: "scheduled", To: "failed"}, {From: "invited", To: "failed"}, {From: "ready", To: "failed"},
		{From: "dispatched", To: "failed"}, {From: "in_progress", To: "failed"}, {From: "completed", To: "failed"},
		{From: "scoring", To: "failed"}, {From: "interrupted", To: "failed"},
	}
}

func seeded(t *testing.T) *statemachine.Machine {
	t.Helper()
	m, err := statemachine.New(seededStates(), seededTransitions())
	require.NoError(t, err)
	return m
}

func TestHappyPath(t *testing.T) {
	m := seeded(t)
	chain := []string{"scheduled", "invited", "ready", "dispatched", "in_progress", "completed", "scoring", "scored"}
	for i := 0; i < len(chain)-1; i++ {
		assert.NoError(t, m.Validate(chain[i], chain[i+1]), "%s -> %s", chain[i], chain[i+1])
	}
}

func TestRejoinEdge(t *testing.T) {
	m := seeded(t)
	assert.NoError(t, m.Validate("dispatched", "interrupted"))
	assert.NoError(t, m.Validate("in_progress", "interrupted"))
	assert.NoError(t, m.Validate("interrupted", "dispatched"))
}

func TestTerminalStatesRejectOutbound(t *testing.T) {
	m := seeded(t)
	for _, term := range []string{"scored", "declined", "abandoned", "failed"} {
		assert.True(t, m.IsTerminal(term))
		err := m.Validate(term, "invited")
		assert.ErrorIs(t, err, statemachine.ErrTerminalState, "from %s", term)
	}
}

func TestIllegalTransitions(t *testing.T) {
	m := seeded(t)
	for _, c := range [][2]string{
		{"scheduled", "completed"},
		{"ready", "scored"},
		{"invited", "in_progress"},
		{"completed", "invited"},
	} {
		err := m.Validate(c[0], c[1])
		assert.ErrorIs(t, err, statemachine.ErrIllegalTransition, "%s -> %s", c[0], c[1])
	}
}

func TestUnknownStates(t *testing.T) {
	m := seeded(t)
	assert.ErrorIs(t, m.Validate("bogus", "invited"), statemachine.ErrUnknownState)
	assert.ErrorIs(t, m.Validate("ready", "bogus"), statemachine.ErrUnknownState)
}

func TestNewRejectsDanglingTransition(t *testing.T) {
	_, err := statemachine.New(
		[]statemachine.State{{Name: "a", Label: "A", IsTerminal: false}, {Name: "b", Label: "B", IsTerminal: false}},
		[]statemachine.Transition{{From: "a", To: "c"}},
	)
	assert.Error(t, err)
}

func TestNewRejectsEdgeOutOfTerminal(t *testing.T) {
	_, err := statemachine.New(
		[]statemachine.State{{Name: "a", Label: "A", IsTerminal: false}, {Name: "z", Label: "Z", IsTerminal: true}},
		[]statemachine.Transition{{From: "z", To: "a"}},
	)
	assert.Error(t, err)
}

func TestGraph(t *testing.T) {
	g := seeded(t).Graph()
	assert.Len(t, g.States, 12)
	assert.Len(t, g.Transitions, 21)
	assert.Equal(t, "abandoned", g.States[0].Name) // sorted
}

func TestLabelAndHas(t *testing.T) {
	m := seeded(t)
	assert.Equal(t, "In progress", m.Label("in_progress"))
	assert.True(t, m.Has("scored"))
	assert.False(t, m.Has("nope"))
}
