// Package statemachine holds the interview-session state graph and validates
// transitions. It is pure: no database, no HTTP. The graph is whatever the
// session_states / session_state_transitions tables hold (docs/ARCHITECTURE.md §4).
package statemachine

import (
	"errors"
	"fmt"
	"sort"
)

// State is one canonical session state.
type State struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	IsTerminal bool   `json:"is_terminal"`
}

// Transition is a legal directed edge in the graph.
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph is the API-facing view of the whole machine.
type Graph struct {
	States      []State      `json:"states"`
	Transitions []Transition `json:"transitions"`
}

// Sentinel errors. Callers map these to transport errors.
var (
	ErrUnknownState      = errors.New("unknown session state")
	ErrIllegalTransition = errors.New("illegal session state transition")
	ErrTerminalState     = errors.New("session state is terminal")
)

// Machine is an immutable, validated view of the state graph.
type Machine struct {
	labels   map[string]string
	terminal map[string]bool
	edges    map[string]map[string]struct{}
	states   []State
}

// New validates that every transition references a known state and returns the
// Machine. A malformed graph is an error (the server should fail startup).
func New(states []State, transitions []Transition) (*Machine, error) {
	if len(states) == 0 {
		return nil, errors.New("statemachine: no states")
	}

	m := &Machine{
		labels:   make(map[string]string, len(states)),
		terminal: make(map[string]bool, len(states)),
		edges:    make(map[string]map[string]struct{}, len(states)),
		states:   append([]State(nil), states...),
	}
	for _, s := range states {
		if s.Name == "" {
			return nil, errors.New("statemachine: state with empty name")
		}
		if _, dup := m.labels[s.Name]; dup {
			return nil, fmt.Errorf("statemachine: duplicate state %q", s.Name)
		}
		m.labels[s.Name] = s.Label
		m.terminal[s.Name] = s.IsTerminal
	}

	for _, t := range transitions {
		if _, ok := m.labels[t.From]; !ok {
			return nil, fmt.Errorf("statemachine: transition from unknown state %q", t.From)
		}
		if _, ok := m.labels[t.To]; !ok {
			return nil, fmt.Errorf("statemachine: transition to unknown state %q", t.To)
		}
		if m.terminal[t.From] {
			return nil, fmt.Errorf("statemachine: transition out of terminal state %q", t.From)
		}
		if m.edges[t.From] == nil {
			m.edges[t.From] = make(map[string]struct{})
		}
		m.edges[t.From][t.To] = struct{}{}
	}

	sort.Slice(m.states, func(i, j int) bool { return m.states[i].Name < m.states[j].Name })
	return m, nil
}

// Has reports whether state is a known state.
func (m *Machine) Has(state string) bool {
	_, ok := m.labels[state]
	return ok
}

// IsTerminal reports whether state has no outgoing transitions by design.
func (m *Machine) IsTerminal(state string) bool { return m.terminal[state] }

// Label returns the display label for state (empty string if unknown).
func (m *Machine) Label(state string) string { return m.labels[state] }

// Validate returns nil when from -> to is a legal edge.
func (m *Machine) Validate(from, to string) error {
	if !m.Has(from) || !m.Has(to) {
		return ErrUnknownState
	}
	if m.terminal[from] {
		return fmt.Errorf("%w: %q", ErrTerminalState, from)
	}
	if _, ok := m.edges[from][to]; !ok {
		return fmt.Errorf("%w: %q -> %q", ErrIllegalTransition, from, to)
	}
	return nil
}

// Graph returns the whole machine, states sorted by name, transitions sorted.
func (m *Machine) Graph() Graph {
	transitions := make([]Transition, 0)
	for from, tos := range m.edges {
		for to := range tos {
			transitions = append(transitions, Transition{From: from, To: to})
		}
	}
	sort.Slice(transitions, func(i, j int) bool {
		if transitions[i].From != transitions[j].From {
			return transitions[i].From < transitions[j].From
		}
		return transitions[i].To < transitions[j].To
	})
	return Graph{States: append([]State(nil), m.states...), Transitions: transitions}
}
