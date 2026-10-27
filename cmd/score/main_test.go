package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var tasks = []Task{{"csvsum", "CSV Column Summer"}, {"lru", "LRU Cache with TTL"}, {"wc", "Word Counter"}}

func TestScore(t *testing.T) {
	l := Ledger{
		Models: map[string]string{"a": "", "b": ""},
		Trials: []Trial{
			{Model: "a", Project: "trials", Title: "A tiny Go module: a word counter with tests", Outcome: "stopped"},
			{Model: "a", Project: "trials", Title: "An LRU cache with TTL (a)", Outcome: "merged"},
			{Model: "a", Project: "trials", Title: "Again: an LRU cache with TTL", Outcome: "blocked"}, // same cell
			{Model: "codex:b", Project: "trials", Title: "Sum a column (csvsum)", Outcome: "done"},
			{Model: "b", Project: "deck", Title: "word counter", Outcome: "merged"},             // another project
			{Model: "claude:opus", Project: "trials", Title: "word counter", Outcome: "merged"}, // not a named model
			{Model: "b", Project: "trials", Title: "something else", Outcome: "merged"},         // no task
		},
	}
	s, cells := Score(l, tasks)
	if len(cells) != 2*len(Harnesses)*3 {
		t.Fatalf("cells = %d, want %d", len(cells), 2*len(Harnesses)*3)
	}
	for _, c := range []Cell{{"a", "open", "wc"}, {"a", "open", "lru"}, {"b", "codex", "csvsum"}} {
		if !cells[c] {
			t.Errorf("cell %v has no verdict", c)
		}
	}
	if want := 3.0 / 30; math.Abs(s-want) > 1e-9 {
		t.Errorf("score = %v, want %v", s, want)
	}
}

func TestScoreNoModels(t *testing.T) {
	if s, _ := Score(Ledger{}, tasks); s != 0 {
		t.Errorf("score = %v, want 0", s)
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

func TestSave(t *testing.T) {
	in := `{"agents": ["x"], "models": {"a": "a at somewhere"}, "trials": [
		{"model": "a", "project": "trials", "title": "word counter", "outcome": "stopped", "task": "1"},
		{"model": "a", "project": "deck", "title": "word counter", "outcome": "merged", "task": "2"}]}`
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := Save(strings.NewReader(in), path); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "deck") || strings.Contains(string(b), "agents") || !strings.Contains(string(b), `"task": "1"`) {
		t.Errorf("saved ledger:\n%s", b)
	}
}
