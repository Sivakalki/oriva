package errors

// InvalidBodyErr wraps a request-body decode failure.
func InvalidBodyErr(err error) error {
	return E(Invalid, "invalid request body", err)
}

// ValidationFailedErr wraps a ValidationErrors value produced by a builder.
func ValidationFailedErr(err error) error {
	return E(Invalid, "validation failed", err)
}

// Unauthorizedf returns a generic Unauthorized error. It is intentionally
// non-specific so callers cannot distinguish "unknown user" from "wrong password".
func Unauthorizedf(msg string) error {
	return E(Unauthorized, msg)
}

// Forbiddenf returns a Forbidden error with the given message.
func Forbiddenf(msg string) error {
	return E(Forbidden, msg)
}
