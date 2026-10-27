package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var tasks = []Task{{"csvsum", "CSV Column Summer"}, {"lru", "LRU Cache with TTL"}, {"wc", "Word Counter"}}

func TestCells(t *testing.T) {
	cells := Cells(Ledger{Models: map[string]string{"a": "", "b": ""}}, tasks)
	if want := (2 + len(Native)) * len(tasks); len(cells) != want {
		t.Fatalf("cells = %d, want %d", len(cells), want)
	}
	for _, c := range cells {
		if c.Model == "a" && c.Harness != "open" {
			t.Errorf("named model a on %s: only open runs a named model", c.Harness)
		}
	}
}

func TestScore(t *testing.T) {
	cells := Cells(Ledger{Models: map[string]string{"a": "", "b": ""}}, tasks)
	ran := map[Cell]Verdict{
		{Pick{"open", "a"}, "wc"}:      {Ran: true, Pass: true},
		{Pick{"open", "b"}, "lru"}:     {Ran: true}, // a failing verdict is a verdict
		{Pick{"codex", "gpt-5"}, "wc"}: {Ran: true, Pass: true},
		{Pick{"claude", "a"}, "wc"}:    {Ran: true, Pass: true}, // not routable
	}
	s, _ := Score(cells, func(c Cell) Verdict { return ran[c] })
	if want := 3.0 / float64(len(cells)); math.Abs(s-want) > 1e-9 {
		t.Errorf("score = %v, want %v", s, want)
	}
	if s, _ := Score(nil, nil); s != 0 {
		t.Errorf("score of no cells = %v, want 0", s)
	}
}

func TestTrialsOf(t *testing.T) {
	l := Ledger{
		Models: map[string]string{"a": ""},
		Trials: []Trial{
			{Task: "1", Model: "a", Project: "trials", Title: "A tiny Go module: a word counter with tests", Outcome: "stopped"},
			{Task: "2", Model: "a", Project: "trials", Title: "Again (wc)", Outcome: "blocked"},
			{Task: "3", Model: "codex:gpt-5", Project: "trials", Title: "An LRU cache with TTL", Outcome: "merged"},
			{Task: "4", Model: "a", Project: "deck", Title: "word counter", Outcome: "merged"},
		},
	}
	got := TrialsOf(l, tasks)
	if n := len(got[Cell{Pick{"open", "a"}, "wc"}]); n != 2 {
		t.Errorf("open:a wc has %d trials, want 2", n)
	}
	if n := len(got[Cell{Pick{"codex", "gpt-5"}, "lru"}]); n != 1 {
		t.Errorf("codex:gpt-5 lru has %d trials, want 1", n)
	}
	if len(got) != 2 {
		t.Errorf("cells with trials = %v, want 2 (another project's trial doesn't count)", got)
	}
}

// trial lays out a repo in a temp dir with the wc task and the cell's code.
func trial(t *testing.T, c Cell, files map[string]string) string {
	root := t.TempDir()
	write := func(path, s string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "tasks/wc/wc_test.go"),
		"//go:build acceptance\n\npackage wc\n\nimport \"testing\"\n\nfunc TestCount(t *testing.T) {\n\tif Count(\"ab\") != 2 {\n\t\tt.Fatal(\"Count(ab) != 2\")\n\t}\n}\n")
	for name, s := range files {
		write(filepath.Join(root, c.Dir(), name), s)
	}
	return root
}

func TestJudge(t *testing.T) {
	c := Cell{Pick{"opencode", "opencode/big-pickle"}, "wc"}
	for _, tc := range []struct {
		name      string
		files     map[string]string
		ran, pass bool
	}{
		{"no code", nil, false, false},
		{"only its own tests", map[string]string{"wc_test.go": "package wc\n"}, false, false},
		{"passes", map[string]string{"wc.go": "package wc\n\nfunc Count(s string) int { return len(s) }\n"}, true, true},
		{"fails", map[string]string{"wc.go": "package wc\n\nfunc Count(s string) int { return 0 }\n"}, true, false},
		{"doesn't build", map[string]string{"wc.go": "package wc\n\nfunc Count(s string) string { return s }\n"}, true, false},
		{"own tests skip the acceptance test", map[string]string{
			"wc.go":      "package wc\n\nfunc Count(s string) int { return 0 }\n",
			"wc_test.go": "package wc\n\nimport \"testing\"\n\nfunc TestCount(t *testing.T) {}\n"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := Judge(trial(t, c, tc.files), c)
			if err != nil {
				t.Fatal(err)
			}
			if v.Ran != tc.ran || v.Pass != tc.pass {
				t.Errorf("verdict = ran %v pass %v, want ran %v pass %v\n%s", v.Ran, v.Pass, tc.ran, tc.pass, v.Out)
			}
		})
	}
}

func TestTasksFromRepo(t *testing.T) {
	got, err := Tasks("../../tasks", "../../TASKS.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no tasks")
	}
	for _, k := range got {
		if k.Name == "" {
			t.Errorf("task %s has no \"## N. Name (%s)\" heading in TASKS.md", k.Dir, k.Dir)
		}
	}
}

// tasks/ holds only acceptance tests, each behind the acceptance tag: no
// answer a trial could copy, and nothing main's go test ./... would build.
func TestNoAnswers(t *testing.T) {
	got, err := Tasks("../../tasks", "../../TASKS.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range got {
		dir := filepath.Join("../../tasks", k.Dir)
		code, err := goFiles(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) > 0 {
			t.Errorf("tasks/%s has code a trial could copy: %v", k.Dir, code)
		}
		tests, err := goFiles(dir, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(tests) == 0 {
			t.Errorf("tasks/%s has no acceptance tests", k.Dir)
		}
		for _, f := range tests {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(b), "//go:build acceptance\n") {
				t.Errorf("%s doesn't start with //go:build acceptance", f)
			}
		}
	}
}

func TestSave(t *testing.T) {
	in := `{"agents": ["x"], "models": {"a": "a at somewhere"}, "trials": [
		{"model": "a", "project": "trials", "title": "word counter", "outcome": "stopped", "task": "1",
		 "reason": "gh: HTTP 403", "cost": 0.5, "minutes": 47, "turns": 7},
		{"model": "a", "project": "deck", "title": "word counter", "outcome": "merged", "task": "2"}]}`
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := Save(strings.NewReader(in), path); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	for _, want := range []string{`"task": "1"`, `"reason": "gh: HTTP 403"`, `"minutes": 47`, `"turns": 7`, `"cost": 0.5`} {
		if !strings.Contains(s, want) {
			t.Errorf("saved ledger has no %s:\n%s", want, s)
		}
	}
	if strings.Contains(s, "deck") || strings.Contains(s, "agents") {
		t.Errorf("saved ledger:\n%s", s)
	}
}
