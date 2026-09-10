package errors

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Error defines a standard application error.
type Error struct {
	// Kind is the transport-agnostic classification of the error.
	Kind Kind `json:"kind"`

	// Message is a human-readable description.
	Message string `json:"message"`

	// WrappedErr is the underlying error, if any.
	WrappedErr error `json:"-"`
}

// Error returns the JSON representation of the error.
func (e *Error) Error() string {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(e)
	return buf.String()
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error { return e.WrappedErr }

// Kind defines the class of an error.
type Kind uint8

const (
	Other             Kind = iota // Unclassified error
	Internal                      // Internal error
	Conflict                      // Entity already exists
	Invalid                       // Invalid input / validation error
	ExpectationFailed             // Expectation failed
	NotFound                      // Entity does not exist
	Unauthorized                  // Unauthorized access
	Forbidden                     // Forbidden access
)

func (k Kind) String() string {
	switch k {
	case Internal:
		return "internal error"
	case Conflict:
		return "conflict"
	case Invalid:
		return "invalid input"
	case ExpectationFailed:
		return "expectation failed"
	case NotFound:
		return "entity not found"
	case Unauthorized:
		return "unauthorized"
	case Forbidden:
		return "forbidden"
	default:
		return "unclassified error"
	}
}

func (k Kind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// E constructs an *Error from a Kind, an error and/or a string, in any order.
func E(args ...any) error {
	e := &Error{}
	for _, arg := range args {
		switch a := arg.(type) {
		case Kind:
			e.Kind = a
		case error:
			e.WrappedErr = a
		case string:
			e.Message = a
		}
	}
	return e
}

var (
	As  = errors.As
	Is  = errors.Is
	New = errors.New
)
