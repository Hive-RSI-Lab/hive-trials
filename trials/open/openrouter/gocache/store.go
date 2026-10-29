package gocache

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Cache is the two folders a check's cache answers from: Shared, filled by
// main's checks and never written here, and Own, this check's. Either may be
// "" for none.
type Cache struct{ Shared, Own string }

// Entry is a cached action: its output's ID, size and file, and when it was
// put.
type Entry struct {
	Output []byte
	Size   int64
	At     time.Time
	Path   string
}

// Get is the action's entry: from Own when this check built it, else from
// Shared.
func (c Cache) Get(action []byte) (Entry, bool) {
	if e, ok := lookup(c.Own, action); ok {
		return e, true
	}
	return lookup(c.Shared, action)
}

// lookup reads an action's entry in dir and checks its output is there; a
// bad or missing entry is a miss, as it is to the go command. An output
// that is a folder is a cached executable, which a GOCACHEPROG never serves.
// A truncated output (size mismatch) is also a miss.
func lookup(dir string, action []byte) (Entry, bool) {
	if dir == "" || len(action) != 32 {
		return Entry{}, false
	}
	raw, err := os.ReadFile(fileName(dir, action, "a"))
	if err != nil {
		return Entry{}, false
	}
	e, ok := parseEntry(raw, action)
	if !ok {
		return Entry{}, false
	}
	e.Path = fileName(dir, e.Output, "d")
	fi, err := os.Lstat(e.Path)
	if err != nil {
		return Entry{}, false
	}
	// Reject if output is a folder (cached executable) or size doesn't match
	if fi.IsDir() || fi.Size() != e.Size {
		return Entry{}, false
	}
	return e, true
}

// Put keeps body in dir, the output before the entry naming it, each written
// whole under a temporary name first: the go commands of one check (vet,
// then test) each run their own copy of this program over the same folder.
func Put(dir string, action, output, body []byte) (string, error) {
	if len(action) != 32 || len(output) != 32 {
		return "", errors.New("bad action or output ID")
	}
	if dir == "" {
		return "", errors.New("no folder to put in")
	}
	path := fileName(dir, output, "d")
	if err := writeFile(path, body); err != nil {
		return "", err
	}
	if err := writeFile(fileName(dir, action, "a"), entryLine(action, output, int64(len(body)), time.Now())); err != nil {
		return "", err
	}
	return path, nil
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	var r [8]byte
	rand.Read(r[:])
	tmp := path + ".tmp-" + hex.EncodeToString(r[:])
	if err := os.WriteFile(tmp, b, 0o666); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
