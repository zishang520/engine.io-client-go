package engine

import "context"

// Error represents a custom error type for Engine.IO transport errors.
// This error type provides additional context and information about transport-related errors.
type Error struct {
	// Message is a human-readable description of the error.
	Message string

	// Description contains the underlying error that caused this transport error.
	Description error

	// Type identifies the category of the error (e.g., "TransportError").
	Type string

	// Context contains additional context information about the error.
	// This can include request/response data, timing information, etc.
	Context context.Context

	// errs contains a slice of underlying errors that contributed to this error.
	// This supports error wrapping and error chain inspection.
	errs []error
}

// Err returns the error interface implementation.
// This allows the Error type to be used as a standard error.
func (e *Error) Err() error {
	return e
}

// Error implements the error interface.
// It returns the human-readable error message.
func (e *Error) Error() string {
	return e.Message
}

// Unwrap returns the slice of underlying errors.
// This implements the errors.Unwrap interface for error chain inspection.
func (e *Error) Unwrap() []error {
	return e.errs
}

// NewTransportError creates a new transport error with the specified details.
//
// Parameters:
//   - reason: A human-readable description of the error
//   - description: The underlying error that caused this transport error
//   - context: Additional context information about the error
//
// Returns: A new Error instance configured as a transport error
func NewTransportError(reason string, description error, context context.Context) *Error {
	return &Error{
		Message:     reason,
		Description: description,
		Type:        "TransportError",
		Context:     context,
		errs:        []error{description},
	}
}
