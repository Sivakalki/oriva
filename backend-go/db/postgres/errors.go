package postgres

import "errors"

// Repo-level sentinels. Services translate these into transport-agnostic
// *apxerrors.Error values.
var (
	// ErrNotFound is returned when a lookup matches no row.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned on a unique-constraint violation.
	ErrConflict = errors.New("conflict")
)

// pgUniqueViolation is the SQLSTATE for a unique_violation.
const pgUniqueViolation = "23505"
