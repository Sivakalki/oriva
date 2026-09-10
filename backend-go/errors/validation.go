package errors

// FieldError is a single field validation failure.
type FieldError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

// ValidationErrors is a list of field errors.
// Prefer ValidationErrs() (the builder) over constructing this directly.
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string { return "validation failed" }

// ValidationErrs returns a new validation error builder.
func ValidationErrs() *ValidationErrorBuilder { return &ValidationErrorBuilder{} }

type ValidationErrorBuilder struct {
	ve ValidationErrors
}

// Add appends a field error.
func (b *ValidationErrorBuilder) Add(field, err string) {
	b.ve = append(b.ve, FieldError{Field: field, Error: err})
}

// Err returns nil when no field errors were added, otherwise the ValidationErrors.
func (b *ValidationErrorBuilder) Err() error {
	if len(b.ve) == 0 {
		return nil
	}
	return b.ve
}
