package config

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var name = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// parse reads r's lines into c; file is what its errors call it.
func parse(r io.Reader, file string, c *Config) error {
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
