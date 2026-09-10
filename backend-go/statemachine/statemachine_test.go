package statemachine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seededStates / seededTransitions mirror migration 0002 (docs/ARCHITECTURE.md §4).
func seededStates() []State {
	return []State{
		{"scheduled", "Scheduled", false},
		{"invited", "Invited", false},
		{"ready", "Ready", false},
		{"dispatched", "Dispatched", false},
		{"in_progress", "In progress", false},
		{"completed", "Completed", false},
		{"scoring", "Scoring", false},
		{"scored", "Scored", true},
		{"declined", "Declined", true},
		{"abandoned", "Abandoned", true},
		{"interrupted", "Interrupted", false},
		{"failed", "Failed", true},
	}
}

func seededTransitions() []Transition {
	return []Transition{
		{"scheduled", "invited"}, {"invited", "ready"}, {"ready", "dispatched"},
		{"dispatched", "in_progress"}, {"in_progress", "completed"},
		{"completed", "scoring"}, {"scoring", "scored"},
		{"ready", "declined"},
		{"invited", "abandoned"}, {"ready", "abandoned"},
		{"dispatched", "interrupted"}, {"in_progress", "interrupted"},
		{"interrupted", "dispatched"},
		{"scheduled", "failed"}, {"invited", "failed"}, {"ready", "failed"},
		{"dispatched", "failed"}, {"in_progress", "failed"}, {"completed", "failed"},
		{"scoring", "failed"}, {"interrupted", "failed"},
	}
}

func seeded(t *testing.T) *Machine {
	t.Helper()
	m, err := New(seededStates(), seededTransitions())
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
		assert.ErrorIs(t, err, ErrTerminalState, "from %s", term)
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
		assert.ErrorIs(t, err, ErrIllegalTransition, "%s -> %s", c[0], c[1])
	}
}

func TestUnknownStates(t *testing.T) {
	m := seeded(t)
	assert.ErrorIs(t, m.Validate("bogus", "invited"), ErrUnknownState)
	assert.ErrorIs(t, m.Validate("ready", "bogus"), ErrUnknownState)
}

func TestNewRejectsDanglingTransition(t *testing.T) {
	_, err := New(
		[]State{{"a", "A", false}, {"b", "B", false}},
		[]Transition{{"a", "c"}},
	)
	assert.Error(t, err)
}

func TestNewRejectsEdgeOutOfTerminal(t *testing.T) {
	_, err := New(
		[]State{{"a", "A", false}, {"z", "Z", true}},
		[]Transition{{"z", "a"}},
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
