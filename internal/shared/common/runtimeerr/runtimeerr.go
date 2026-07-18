package runtimeerr

// Error is a domain error that can cross HTTP and tunnel boundaries with a stable code
// and a client-safe message. Internal causes must not be placed in RuntimeErrorMessage.
type Error interface {
	error
	RuntimeErrorCode() string
	RuntimeErrorMessage() string
}
