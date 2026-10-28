//go:build acceptance

package gocache

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// id is a 32-byte ID whose first byte is first, the rest from seed.
func id(first byte, seed string) []byte {
	h := sha256.Sum256([]byte(seed))
	h[0] = first
	return h[:]
}

// goPut writes an entry into dir as the go command does with a plain
// GOCACHE: <dir>/<first byte, two hex digits>/<id>-a and -d.
func goPut(t *testing.T, dir string, action, output, body []byte) {
	t.Helper()
	write := func(i []byte, key string, b []byte) {
		p := filepath.Join(dir, fmt.Sprintf("%02x", i[0]), fmt.Sprintf("%x-%s", i, key))
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write(output, "d", body)
	write(action, "a", []byte(fmt.Sprintf("v1 %x %x %20d %20d\n", action, output, len(body), time.Now().UnixNano())))
}

type res struct {
	ID            int64
	Err           string
	KnownCommands []string
	Miss          bool
	OutputID      []byte
	Size          int64
	Time          *time.Time
	DiskPath      string
}

// serve runs c.Serve over the requests (each a JSON value, a put's body
// after it) and returns its answers, the handshake first. It fails the test
// if Serve doesn't return within 5s.
func serve(t *testing.T, c Cache, reqs ...any) []res {
	t.Helper()
	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	for _, r := range reqs {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := c.Serve(&in, pw)
		pw.Close()
		done <- err
	}()
	var out []res
	read := make(chan struct{})
	go func() {
		defer close(read)
		sc := bufio.NewScanner(pr)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			var r res
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				t.Errorf("answer %q: %v", sc.Text(), err)
				continue
			}
			out = append(out, r)
		}
	}()
	select {
	case err := <-done:
		<-read
		if err != nil {
			t.Fatalf("Serve: %v (answers so far: %+v)", err, out)
		}
	case <-time.After(5 * time.Second):
		pr.Close()
		t.Fatal("Serve didn't return within 5s")
	}
	return out
}

type get struct {
	ID       int64
	Command  string
	ActionID []byte
}

type put struct {
	ID       int64
	Command  string
	ActionID []byte
	OutputID []byte
	BodySize int64
}

type cmd struct {
	ID      int64
	Command string
}

func TestHandshake(t *testing.T) {
	out := serve(t, Cache{Own: t.TempDir()})
	if len(out) != 1 {
		t.Fatalf("answers = %+v, want just the handshake", out)
	}
	got := strings.Join(out[0].KnownCommands, ",")
	for _, c := range []string{"get", "put", "close"} {
		if !strings.Contains(","+got+",", ","+c+",") {
			t.Errorf("KnownCommands = %v, want %s in it", out[0].KnownCommands, c)
		}
	}
}

func TestPutThenGet(t *testing.T) {
	c := Cache{Shared: t.TempDir(), Own: t.TempDir()}
	a, o := id(0xc4, "a"), id(0x7e, "o")
	body := []byte("compiled object")
	out := serve(t, c,
		put{1, "put", a, o, int64(len(body))}, body,
		get{2, "get", a},
		cmd{3, "close"})
	if len(out) != 4 {
		t.Fatalf("answers = %+v, want handshake, put, get, close", out)
	}
	p, g := out[1], out[2]
	if p.ID != 1 || p.Err != "" || p.DiskPath == "" {
		t.Fatalf("put answer = %+v", p)
	}
	if b, err := os.ReadFile(p.DiskPath); err != nil || !bytes.Equal(b, body) {
		t.Errorf("put's DiskPath holds %q, %v; want %q", b, err, body)
	}
	if g.ID != 2 || g.Miss || !bytes.Equal(g.OutputID, o) || g.Size != int64(len(body)) || g.Time == nil {
		t.Fatalf("get answer = %+v, want a hit on output %x size %d", g, o, len(body))
	}
	if b, err := os.ReadFile(g.DiskPath); err != nil || !bytes.Equal(b, body) {
		t.Errorf("get's DiskPath holds %q, %v; want %q", b, err, body)
	}
	if out[3].ID != 3 {
		t.Errorf("close answer = %+v", out[3])
	}
	if e, err := os.ReadDir(c.Shared); err != nil || len(e) != 0 {
		t.Errorf("the shared folder was written: %v %v", e, err)
	}
}

// Bug report: a check hangs, or answers go's next request wrongly, after go
// puts an empty output (BodySize 0, which go sends with no body after it).
func TestEmptyPut(t *testing.T) {
	c := Cache{Own: t.TempDir()}
	a, o := id(0x51, "empty"), id(0x52, "empty out")
	out := serve(t, c,
		put{1, "put", a, o, 0},
		get{2, "get", a},
		cmd{3, "close"})
	if len(out) != 4 {
		t.Fatalf("answers = %+v, want handshake, put, get, close", out)
	}
	if out[1].ID != 1 || out[1].Err != "" {
		t.Errorf("put answer = %+v", out[1])
	}
	g := out[2]
	if g.ID != 2 || g.Miss || !bytes.Equal(g.OutputID, o) || g.Size != 0 {
		t.Errorf("get answer = %+v, want a hit on the empty output", g)
	}
	if out[3].ID != 3 {
		t.Errorf("close answer = %+v", out[3])
	}
}

func TestBodySizeMismatch(t *testing.T) {
	c := Cache{Own: t.TempDir()}
	out := serve(t, c, put{1, "put", id(1, "a"), id(2, "o"), 9}, []byte("short"), cmd{2, "close"})
	if len(out) != 3 || out[1].Err == "" {
		t.Errorf("answers = %+v, want an error for the put", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	out := serve(t, Cache{Own: t.TempDir()}, cmd{1, "frobnicate"}, cmd{2, "close"})
	if len(out) != 3 || out[1].ID != 1 || out[1].Err == "" {
		t.Errorf("answers = %+v, want an error for the unknown command", out)
	}
}

func TestEOFEnds(t *testing.T) {
	out := serve(t, Cache{Own: t.TempDir()}, get{1, "get", id(3, "x")})
	if len(out) != 2 || !out[1].Miss {
		t.Errorf("answers = %+v, want the handshake and a miss", out)
	}
}

// Bug report: a branch check gets main's output for an action it rebuilt
// itself, when both have it: what this check built must win.
func TestOwnBeforeShared(t *testing.T) {
	c := Cache{Shared: t.TempDir(), Own: t.TempDir()}
	a := id(0xab, "both")
	goPut(t, c.Shared, a, id(0xcd, "main's"), []byte("main's build"))
	mine := id(0xef, "mine")
	if _, err := Put(c.Own, a, mine, []byte("this check's build")); err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get(a)
	if !ok || !bytes.Equal(e.Output, mine) {
		t.Fatalf("Get = %x, %v; want this check's output %x", e.Output, ok, mine)
	}
	if b, _ := os.ReadFile(e.Path); string(b) != "this check's build" {
		t.Errorf("Get's Path holds %q", b)
	}
}

// Bug report: about one action in sixteen always misses in main's shared
// cache though go wrote it there: those whose ID starts with a byte below
// 0x10, which go keeps in a folder named with two hex digits ("0a").
func TestSharedLayout(t *testing.T) {
	c := Cache{Shared: t.TempDir(), Own: t.TempDir()}
	for _, first := range []byte{0x00, 0x0a, 0x0f, 0x10, 0xff} {
		a, o := id(first, "action"), id(first, "output")
		body := []byte(fmt.Sprintf("built %02x", first))
		goPut(t, c.Shared, a, o, body)
		e, ok := c.Get(a)
		if !ok || !bytes.Equal(e.Output, o) || e.Size != int64(len(body)) {
			t.Errorf("action %02x…: Get = %+v, %v; want a hit on go's entry", first, e, ok)
			continue
		}
		if b, _ := os.ReadFile(e.Path); !bytes.Equal(b, body) {
			t.Errorf("action %02x…: Path holds %q, want %q", first, b, body)
		}
	}
	// What a check puts in its own folder is laid out as go lays it out too.
	a, o := id(0x05, "own"), id(0x06, "own out")
	if _, err := Put(c.Own, a, o, []byte("x")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{fmt.Sprintf("05/%x-a", a), fmt.Sprintf("06/%x-d", o)} {
		if _, err := os.Stat(filepath.Join(c.Own, p)); err != nil {
			t.Errorf("Put left no %s: %v", p, err)
		}
	}
}

// Bug report: an output file cut short (a full disk, a killed copy) is
// served, and the build fails on a corrupt object instead of rebuilding.
func TestBadOutputMisses(t *testing.T) {
	c := Cache{Shared: t.TempDir()}
	a, o := id(0x21, "cut"), id(0x22, "cut out")
	goPut(t, c.Shared, a, o, []byte("the whole object"))
	p := filepath.Join(c.Shared, "22", fmt.Sprintf("%x-d", o))
	if err := os.WriteFile(p, []byte("the wh"), 0o666); err != nil {
		t.Fatal(err)
	}
	if e, ok := c.Get(a); ok {
		t.Errorf("Get of a cut-short output = %+v, want a miss", e)
	}

	// An output that is a folder is a cached executable: never served.
	a2, o2 := id(0x31, "exe"), id(0x32, "exe out")
	goPut(t, c.Shared, a2, o2, nil)
	p2 := filepath.Join(c.Shared, "32", fmt.Sprintf("%x-d", o2))
	if err := os.Remove(p2); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p2, 0o777); err != nil {
		t.Fatal(err)
	}
	if e, ok := c.Get(a2); ok {
		t.Errorf("Get of an output that is a folder = %+v, want a miss", e)
	}

	// A missing output is a miss.
	a3, o3 := id(0x41, "gone"), id(0x42, "gone out")
	goPut(t, c.Shared, a3, o3, []byte("x"))
	os.Remove(filepath.Join(c.Shared, "42", fmt.Sprintf("%x-d", o3)))
	if e, ok := c.Get(a3); ok {
		t.Errorf("Get of a missing output = %+v, want a miss", e)
	}
}

func TestBadEntryMisses(t *testing.T) {
	c := Cache{Shared: t.TempDir()}
	a, o := id(0x61, "entry"), id(0x62, "entry out")
	goPut(t, c.Shared, a, o, []byte("x"))
	ap := filepath.Join(c.Shared, "61", fmt.Sprintf("%x-a", a))
	for name, line := range map[string]string{
		"other version":   fmt.Sprintf("v2 %x %x %20d %20d\n", a, o, 1, 1),
		"other action":    fmt.Sprintf("v1 %x %x %20d %20d\n", id(0x61, "else"), o, 1, 1),
		"no newline":      fmt.Sprintf("v1 %x %x %20d %20d", a, o, 1, 1),
		"short output ID": fmt.Sprintf("v1 %x %x %20d %20d\n", a, o[:16], 1, 1),
		"bad size":        fmt.Sprintf("v1 %x %x %20s %20d\n", a, o, "one", 1),
		"negative size":   fmt.Sprintf("v1 %x %x %20d %20d\n", a, o, -1, 1),
		"too few fields":  fmt.Sprintf("v1 %x %x %20d\n", a, o, 1),
	} {
		if err := os.WriteFile(ap, []byte(line), 0o666); err != nil {
			t.Fatal(err)
		}
		if e, ok := c.Get(a); ok {
			t.Errorf("%s: Get = %+v, want a miss", name, e)
		}
	}
	if _, ok := c.Get(id(0x61, "never put")); ok {
		t.Error("Get of an action never put hit")
	}
	if _, ok := c.Get(a[:16]); ok {
		t.Error("Get of a short action ID hit")
	}
	if _, ok := (Cache{}).Get(a); ok {
		t.Error("Get with no folders hit")
	}
}

func TestPutRejectsBadIDs(t *testing.T) {
	if _, err := Put(t.TempDir(), id(1, "a")[:31], id(2, "o"), nil); err == nil {
		t.Error("Put with a short action ID: no error")
	}
	if _, err := Put(t.TempDir(), id(1, "a"), nil, nil); err == nil {
		t.Error("Put with no output ID: no error")
	}
}
