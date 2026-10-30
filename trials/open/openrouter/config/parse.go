package config

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var name = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// parse reads r's lines into c; file is what its errors call it.
// includeStack tracks files being parsed to detect cycles.
func parse(r io.Reader, file string, c *Config, includeStack []string) error {
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
		case line == "include" || strings.HasPrefix(line, "include "):
			// Handle include directive - must be exactly "include" or "include " at start
			// But not "include = ..." which is a key-value pair
			rest := strings.TrimSpace(line[len("include"):])
			if rest == "" {
				return errorf(file, n, "include: missing path")
			}
			if strings.HasPrefix(rest, "=") {
				// This is "include = ..." - a key-value pair, not an include directive
				// Let it fall through to the key-value parsing below
			} else {
				// Parse path like a value (quoted or bare with optional comment)
				includePath, err := value(rest)
				if err != nil {
					return errorf(file, n, "include: %v", err)
				}
				if err := includeFile(includePath, file, c, includeStack, n); err != nil {
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
		if !name.MatchString(k) {
			return errorf(file, n, "bad key %q", k)
		}
		val, err := value(strings.TrimSpace(v))
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

// includeFile reads and parses an included config file.
func includeFile(includePath, currentFile string, c *Config, includeStack []string, includeLine int) error {
	// Resolve path relative to current file's directory
	dir := filepath.Dir(currentFile)
	var absPath string
	if filepath.IsAbs(includePath) {
		absPath = includePath
	} else {
		absPath = filepath.Join(dir, includePath)
	}
	// Normalize path for cycle detection
	cleanPath := filepath.Clean(absPath)

	// Check for cycles
	for _, f := range includeStack {
		if f == cleanPath {
			return errorf(currentFile, includeLine, "include cycle: %s", cleanPath)
		}
	}

	// Open and parse the included file
	f, err := os.Open(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errorfWithErr(currentFile, includeLine, err, "include %q: %v", includePath, err)
		}
		return errorf(currentFile, includeLine, "include %q: %v", includePath, err)
	}
	defer f.Close()

	// Check if it's a directory
	fi, err := f.Stat()
	if err == nil && fi.IsDir() {
		return errorf(currentFile, includeLine, "include %q: is a directory", includePath)
	}

	// Parse with updated include stack
	newStack := append(includeStack, cleanPath)
	return parse(f, cleanPath, c, newStack)
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
