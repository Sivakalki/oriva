// Package state_repo loads the canonical session-state graph (the
// session_states / session_state_transitions catalog tables) that the
// statemachine package validates against. Go is the sole authority for
// these tables (docs/ARCHITECTURE.md §4).
package state_repo

import (
	"context"
	"fmt"

	"oriva/backend-go/statemachine"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LoadGraph reads the canonical state list and legal transition graph from
// Postgres.
func LoadGraph(
	ctx context.Context, pool *pgxpool.Pool,
) ([]statemachine.State, []statemachine.Transition, error) {
	stateRows, err := pool.Query(ctx,
		`SELECT name, label, is_terminal FROM session_states ORDER BY name`)
	if err != nil {
		return nil, nil, fmt.Errorf("query session_states: %w", err)
	}
	defer stateRows.Close()

	var states []statemachine.State
	for stateRows.Next() {
		var s statemachine.State
		if err := stateRows.Scan(&s.Name, &s.Label, &s.IsTerminal); err != nil {
			return nil, nil, err
		}
		states = append(states, s)
	}
	if err := stateRows.Err(); err != nil {
		return nil, nil, err
	}

	transitionRows, err := pool.Query(ctx,
		`SELECT from_state, to_state FROM session_state_transitions ORDER BY from_state, to_state`)
	if err != nil {
		return nil, nil, fmt.Errorf("query session_state_transitions: %w", err)
	}
	defer transitionRows.Close()

	var transitions []statemachine.Transition
	for transitionRows.Next() {
		var t statemachine.Transition
		if err := transitionRows.Scan(&t.From, &t.To); err != nil {
			return nil, nil, err
		}
		transitions = append(transitions, t)
	}
	if err := transitionRows.Err(); err != nil {
		return nil, nil, err
	}

	return states, transitions, nil
}
