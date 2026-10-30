//go:build acceptance

package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// files writes each name's text under a new temp dir and returns the dir.
func files(t *testing.T, fs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range fs {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// load runs Load(path), failing the test if it doesn't return within 5s.
func load(t *testing.T, path string) (*Config, error) {
	t.Helper()
	type res struct {
		c   *Config
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := Load(path)
		ch <- res{c, err}
	}()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-time.After(5 * time.Second):
		t.Fatalf("Load(%s) didn't return in 5s", path)
		return nil, nil
	}
}

// values are every key's value.
func values(c *Config) map[string]string {
	out := map[string]string{}
	for _, k := range c.Keys() {
		out[k], _ = c.Get(k)
	}
	return out
}

func wantValues(t *testing.T, path string, want map[string]string) {
	t.Helper()
	c, err := load(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := values(c); !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
}

// wantError checks err is an *Error at file (by its last path elements)
// and line.
func wantError(t *testing.T, err error, file string, line int) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("err = %v (%T), want an *Error at %s:%d", err, err, file, line)
	}
	if filepath.ToSlash(e.File) != file && !strings.HasSuffix(filepath.ToSlash(e.File), "/"+file) {
		t.Errorf("error file = %q, want %s (%v)", e.File, file, err)
	}
	if e.Line != line {
		t.Errorf("error line = %d, want %d (%v)", e.Line, line, err)
	}
	if want := filepath.Base(file) + ":" + itoa(line) + ": "; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q doesn't say %q", err.Error(), want)
	}
	return e
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

// What loads now loads the same.

func TestPlainFile(t *testing.T) {
	dir := files(t, map[string]string{"a.conf": `
# comment
; also a comment
name = api
greeting = "hello, world"   # quoted keeps its spaces
bare = one two # comment
hash = a#b
empty =
dup = first
dup = second
[db]
host = db.internal
port=5432
[]
top = again
`})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{
		"name": "api", "greeting": "hello, world", "bare": "one two", "hash": "a#b",
		"empty": "", "dup": "second", "db.host": "db.internal", "db.port": "5432", "top": "again",
	})
}

func TestIncludeNamedKeysStayKeys(t *testing.T) {
	dir := files(t, map[string]string{"a.conf": `
include = yes
include=no-spaces
includes = 2
[include]
path = /etc/x
[build]
include = src
`})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{
		"include": "no-spaces", "includes": "2", "include.path": "/etc/x", "build.include": "src",
	})
}

func TestPlainErrors(t *testing.T) {
	for _, tc := range []struct {
		text string
		line int
	}{
		{"a = 1\njunk\n", 2},
		{"a = 1\n\n[db\n", 3},
		{"bad key = 1\n", 1},
		{"a = \"open\n", 1},
		{"a = \"x\" y\n", 1},
	} {
		dir := files(t, map[string]string{"a.conf": tc.text})
		_, err := load(t, filepath.Join(dir, "a.conf"))
		wantError(t, err, "a.conf", tc.line)
	}
}

func TestMissingFile(t *testing.T) {
	_, err := load(t, filepath.Join(t.TempDir(), "none.conf"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

// Includes.

func TestIncludeInPlace(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf": "x = before\ny = before\ninclude b.conf\ny = after\n",
		"b.conf": "x = b\ny = b\nz = b\n",
	})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{"x": "b", "y": "after", "z": "b"})
}

func TestIncludeQuotedAndCommented(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf":           "include \"my settings.conf\"  # quoted\ninclude   b.conf # bare\n",
		"my settings.conf": "s = spaced\n",
		"b.conf":           "b = bare\n",
	})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{"s": "spaced", "b": "bare"})
}

func TestIncludeAbsolute(t *testing.T) {
	other := files(t, map[string]string{"b.conf": "b = abs\n"})
	dir := files(t, map[string]string{"a.conf": "include " + filepath.Join(other, "b.conf") + "\n"})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{"b": "abs"})
}

func TestIncludeRelativeToIncluder(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf":            "include sub/b.conf\n",
		"sub/b.conf":        "include c.conf\ninclude ../d.conf\ninclude deeper/e.conf\n",
		"sub/c.conf":        "c = sub\n",
		"c.conf":            "c = top\n",
		"d.conf":            "d = top\n",
		"sub/deeper/e.conf": "e = deeper\n",
		"deeper/e.conf":     "e = wrong\n",
	})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{"c": "sub", "d": "top", "e": "deeper"})
}

func TestIncludeLoadedByRelativePath(t *testing.T) {
	dir := files(t, map[string]string{
		"conf/a.conf": "include b.conf\n",
		"conf/b.conf": "b = 1\n",
	})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	wantValues(t, filepath.Join("conf", "a.conf"), map[string]string{"b": "1"})
}

func TestParseIncludesRelativeToName(t *testing.T) {
	dir := files(t, map[string]string{"etc/b.conf": "b = 1\n"})
	c, err := Parse(strings.NewReader("a = 0\ninclude b.conf\n"), filepath.Join(dir, "etc", "a.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if got := values(c); !reflect.DeepEqual(got, map[string]string{"a": "0", "b": "1"}) {
		t.Errorf("values = %v", got)
	}
}

func TestIncludeSections(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf": "[db]\nhost = h\ninclude b.conf\nport = 1\n",
		"b.conf": "top = b\n[cache]\nsize = 9\n",
	})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{
		"db.host": "h", "top": "b", "cache.size": "9", "db.port": "1",
	})
}

func TestIncludeTwiceIsNoCycle(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf":      "n = a\ninclude b.conf\ninclude c.conf\ninclude common.conf\n",
		"b.conf":      "include common.conf\nb = 1\n",
		"c.conf":      "include ./common.conf\nc = 1\n",
		"common.conf": "common = yes\nn = common\n",
	})
	wantValues(t, filepath.Join(dir, "a.conf"), map[string]string{"n": "common", "b": "1", "c": "1", "common": "yes"})
}

func TestIncludeCycles(t *testing.T) {
	for name, fs := range map[string]map[string]string{
		"self":          {"a.conf": "x = 1\ninclude a.conf\n"},
		"self dotted":   {"a.conf": "include ./a.conf\n"},
		"through two":   {"a.conf": "include b.conf\n", "b.conf": "include a.conf\n"},
		"spelled apart": {"a.conf": "include sub/b.conf\n", "sub/b.conf": "include ../sub/../a.conf\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := files(t, fs)
			if _, err := load(t, filepath.Join(dir, "a.conf")); err == nil {
				t.Error("Load of a cycle: no error")
			}
		})
	}
	t.Run("absolute", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a.conf")
		if err := os.WriteFile(a, []byte("include b.conf\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.conf"), []byte("include "+a+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := load(t, a); err == nil {
			t.Error("Load of a cycle: no error")
		}
	})
}

func TestIncludedErrorNamesItsFile(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf":     "a = 1\n\ninclude sub/b.conf\n",
		"sub/b.conf": "# b\nb = 1\njunk\n",
	})
	_, err := load(t, filepath.Join(dir, "a.conf"))
	wantError(t, err, "sub/b.conf", 3)
}

func TestMissingInclude(t *testing.T) {
	dir := files(t, map[string]string{
		"a.conf": "a = 1\ninclude b.conf\n",
		"b.conf": "b = 1\n\n\ninclude none.conf\n",
	})
	_, err := load(t, filepath.Join(dir, "a.conf"))
	wantError(t, err, "b.conf", 4)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("errors.Is(%v, fs.ErrNotExist) = false", err)
	}
}

func TestIncludeNothing(t *testing.T) {
	for _, text := range []string{"a = 1\ninclude\n", "a = 1\ninclude   \n", "a = 1\ninclude \"\"\n"} {
		dir := files(t, map[string]string{"a.conf": text})
		_, err := load(t, filepath.Join(dir, "a.conf"))
		wantError(t, err, "a.conf", 2)
	}
}

func TestIncludeFolder(t *testing.T) {
	dir := files(t, map[string]string{"a.conf": "include sub\n", "sub/x.conf": "x = 1\n"})
	_, err := load(t, filepath.Join(dir, "a.conf"))
	wantError(t, err, "a.conf", 1)
}
