package domain

import "fmt"

// CLIError is a domain error carrying a stable code and the process exit code
// it should map to (SPECS §7.3).
type CLIError struct {
	Code     string
	Msg      string
	ExitCode int
	Cause    error
}

func (e *CLIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Msg, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *CLIError) Unwrap() error { return e.Cause }

// Predefined CLI errors mapped to exit codes (SPECS §7.3).
var (
	ErrGeneral = func(msg string, cause error) *CLIError {
		return &CLIError{Code: "general", Msg: msg, ExitCode: 1, Cause: cause}
	}
	ErrConfig = func(msg string, cause error) *CLIError {
		return &CLIError{Code: "config", Msg: msg, ExitCode: 2, Cause: cause}
	}
	ErrPlatformNotReady = func(msg string, cause error) *CLIError {
		return &CLIError{Code: "not_ready", Msg: msg, ExitCode: 3, Cause: cause}
	}
	ErrConflict = func(msg string, cause error) *CLIError {
		return &CLIError{Code: "conflict", Msg: msg, ExitCode: 4, Cause: cause}
	}
	ErrAuth = func(msg string, cause error) *CLIError {
		return &CLIError{Code: "auth", Msg: msg, ExitCode: 5, Cause: cause}
	}
)

// ExitCodeOf returns the exit code for an error, defaulting to 1.
func ExitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if ce, ok := err.(*CLIError); ok {
		return ce.ExitCode
	}
	return 1
}
