package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var name = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// parse reads r's lines into c; file is what its errors call it.
func parse(r io.Reader, file string, c *Config) error {
	return parseWithVisited(r, file, c, nil)
}

func parseWithVisited(r io.Reader, file string, c *Config, visited map[string]bool) error {
	sc := bufio.NewScanner(r)
	section := ""
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || line[0] == '#' || line[0] == ';':
			continue
		case line[0] == '[':
			if !strings.HasSuffix(line, "]") {
				return errorf(file, n, "unclosed section header")
			}
			s := strings.TrimSpace(line[1 : len(line)-1])
			if s != "" && !name.MatchString(s) {
				return errorf(file, n, "bad section name %q", s)
			}
			section = s
			continue
		}
		// Check for include directive BEFORE splitting on =
		// Only match "include " (with space) or "include" alone, not "include=" which is an assignment
		if strings.HasPrefix(line, "include ") || line == "include" {
			// Make sure this isn't "include = something" or "include=something" (an assignment)
			rest := strings.TrimSpace(line[7:])
			if !strings.HasPrefix(rest, "=") {
				if err := handleInclude(rest, file, n, c, section, visited); err != nil {
					return err
				}
				continue
			}
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return errorf(file, n, "want key = value")
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !name.MatchString(k) {
			return errorf(file, n, "bad key %q", k)
		}
		val, err := value(v)
		if err != nil {
			return errorf(file, n, "%s: %v", k, err)
		}
		if section != "" {
			k = section + "." + k
		}
		c.values[k] = val
	}
	return sc.Err()
}

func handleInclude(v, file string, line int, c *Config, section string, visited map[string]bool) error {
	if v == "" {
		return errorf(file, line, "include: empty path")
	}
	// Handle quoted path
	path := v
	if strings.HasPrefix(v, `"`) {
		q, err := strconv.QuotedPrefix(v)
		if err != nil {
			return errorf(file, line, "include: unclosed quote")
		}
		if rest := strings.TrimSpace(v[len(q):]); rest != "" && rest[0] != '#' {
			return errorf(file, line, "include: text after the closing quote")
		}
		path, err = strconv.Unquote(q)
		if err != nil {
			return errorf(file, line, "include: %v", err)
		}
	} else {
		// Bare path: cut at " #" comment
		if i := strings.Index(v, " #"); i >= 0 {
			path = strings.TrimSpace(v[:i])
		}
	}
	if path == "" {
		return errorf(file, line, "include: empty path")
	}

	// Resolve path relative to the includer's directory
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(filepath.Dir(file), path)
	}

	// Normalize to check for cycles
	cleanPath := filepath.Clean(absPath)

	// Check for cycles
	if visited == nil {
		visited = make(map[string]bool)
	}
	if visited[cleanPath] {
		return errorf(file, line, "include: cycle detected")
	}
	visited[cleanPath] = true
	defer delete(visited, cleanPath)

	// Open and parse the included file
	f, err := os.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Error{File: file, Line: line, Msg: fmt.Sprintf("include %q: %v", path, err), Err: err}
		}
		return &Error{File: file, Line: line, Msg: fmt.Sprintf("include %q: %v", path, err), Err: err}
	}
	defer f.Close()

	// Check if it's a directory
	fi, err := f.Stat()
	if err != nil {
		return &Error{File: file, Line: line, Msg: fmt.Sprintf("include %q: %v", path, err), Err: err}
	}
	if fi.IsDir() {
		return &Error{File: file, Line: line, Msg: fmt.Sprintf("include %q: is a directory", path)}
	}

	return parseWithVisited(f, cleanPath, c, visited)
}

// value is a value as written: "quoted", read as a Go string, or bare, up
// to a " #" comment; spaces around either are dropped.
func value(v string) (string, error) {
	if strings.HasPrefix(v, `"`) {
		q, err := strconv.QuotedPrefix(v)
		if err != nil {
			return "", errors.New("unclosed quote")
		}
		if rest := strings.TrimSpace(v[len(q):]); rest != "" && rest[0] != '#' {
			return "", errors.New("text after the closing quote")
		}
		return strconv.Unquote(q)
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v), nil
}