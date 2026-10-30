package config

import "fmt"

// Error is a problem at one line of a config file: its Error() reads
// "file:line: what".
type Error struct {
	File string
	Line int
	Msg  string
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg) }

// Unwrap returns the underlying error for errors.Is/As
func (e *Error) Unwrap() error { return e.Err }

// errorf is an *Error at line n of file.
func errorf(file string, n int, format string, args ...any) *Error {
	return &Error{File: file, Line: n, Msg: fmt.Sprintf(format, args...)}
}

// errorfWithErr wraps an error in an *Error at line n of file.
func errorfWithErr(file string, n int, err error, format string, args ...any) *Error {
	return &Error{File: file, Line: n, Msg: fmt.Sprintf(format, args...), Err: err}
}
