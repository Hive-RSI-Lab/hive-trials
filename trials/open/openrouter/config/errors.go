package config

import "fmt"

// Error is a problem at one line of a config file: its Error() reads
// "file:line: what".
type Error struct {
	File string
	Line int
	Msg  string
	Err  error // underlying error, if any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s:%d: %s: %v", e.File, e.Line, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

func (e *Error) Unwrap() error { return e.Err }

// errorf is an *Error at line n of file.
func errorf(file string, n int, format string, args ...any) *Error {
	return &Error{File: file, Line: n, Msg: fmt.Sprintf(format, args...)}
}
