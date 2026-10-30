package config

import "fmt"

// Error is a problem at one line of a config file: its Error() reads
// "file:line: what".
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg) }

// errorf is an *Error at line n of file.
func errorf(file string, n int, format string, args ...any) *Error {
	return &Error{File: file, Line: n, Msg: fmt.Sprintf(format, args...)}
}
