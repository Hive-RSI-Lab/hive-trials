// Package gocache is the Go build cache a check sees: a GOCACHEPROG
// (https://pkg.go.dev/cmd/go/internal/cacheprog) that answers from two
// folders laid out as the go command lays out GOCACHE.
//
// The shared folder is read-only to it: only checks of a project's own main
// fill it (with a plain GOCACHE), so a check of any other commit reuses what
// main's checks built and can't plant anything for the next check to trust.
// What this check builds goes in its own folder.
//
// This is a copy of the Hive's internal/gocache, split into serve.go (the
// protocol), store.go (reading and writing entries) and layout.go (where an
// entry lives and what it says).
package gocache

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"
)

type request struct {
	ID       int64
	Command  string
	ActionID []byte `json:",omitempty"`
	OutputID []byte `json:",omitempty"`
	BodySize int64  `json:",omitempty"`
}

type response struct {
	ID            int64
	Err           string     `json:",omitempty"`
	KnownCommands []string   `json:",omitempty"`
	Miss          bool       `json:",omitempty"`
	OutputID      []byte     `json:",omitempty"`
	Size          int64      `json:",omitempty"`
	Time          *time.Time `json:",omitempty"`
	DiskPath      string     `json:",omitempty"`
}

// Serve answers the go command's requests on in until it closes or asks to.
// A put's body, when it has one (BodySize > 0), follows its request as a
// base64 JSON string of its own.
func (c Cache) Serve(in io.Reader, out io.Writer) error {
	w := bufio.NewWriter(out)
	enc := json.NewEncoder(w)
	send := func(r response) error {
		if err := enc.Encode(r); err != nil {
			return err
		}
		return w.Flush()
	}
	if err := send(response{KnownCommands: []string{"get", "put", "close"}}); err != nil {
		return err
	}
	dec := json.NewDecoder(bufio.NewReader(in))
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		res := response{ID: req.ID}
		switch req.Command {
		case "get":
			if e, ok := c.Get(req.ActionID); ok {
				res.OutputID, res.Size, res.Time, res.DiskPath = e.Output, e.Size, &e.At, e.Path
			} else {
				res.Miss = true
			}
		case "put":
			var body []byte
			if req.BodySize > 0 {
				if err := dec.Decode(&body); err != nil {
					return err
				}
			}
			if int64(len(body)) != req.BodySize {
				res.Err = "body size mismatch"
			} else if path, err := Put(c.Own, req.ActionID, req.OutputID, body); err != nil {
				res.Err = err.Error()
			} else {
				res.DiskPath = path
			}
		case "close":
			return send(res)
		default:
			res.Err = "unknown command " + strconv.Quote(req.Command)
		}
		if err := send(res); err != nil {
			return err
		}
	}
}
