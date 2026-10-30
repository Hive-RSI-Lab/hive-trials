package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
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
		{Pick{"open", "a"}, "wc"}:   {Ran: true, Pass: true},
		{Pick{"open", "b"}, "lru"}:  {Ran: true}, // a failing verdict is a verdict
		{Pick{"codex", ""}, "wc"}:   {Ran: true, Pass: true},
		{Pick{"claude", "a"}, "wc"}: {Ran: true, Pass: true}, // not routable
	}
	all := func(Cell) bool { return true }
	s, _ := Score(cells, all, func(c Cell) Verdict { return ran[c] })
	if want := 3.0 / float64(len(cells)); math.Abs(s-want) > 1e-9 {
		t.Errorf("score = %v, want %v", s, want)
	}
	// Code no trial of its cell landed doesn't count, and isn't judged.
	landed := func(c Cell) bool { return c != Cell{Pick{"open", "b"}, "lru"} }
	s, vs := Score(cells, landed, func(c Cell) Verdict {
		if !landed(c) {
			t.Errorf("judged %v, which no trial landed", c)
		}
		return ran[c]
	})
	if want := 2.0 / float64(len(cells)); math.Abs(s-want) > 1e-9 {
		t.Errorf("score with open:b lru not landed = %v, want %v", s, want)
	}
	if vs[Cell{Pick{"open", "b"}, "lru"}].Ran {
		t.Errorf("open:b lru has a verdict though no trial landed it")
	}
	if s, _ := Score(nil, nil, nil); s != 0 {
		t.Errorf("score of no cells = %v, want 0", s)
	}
}

func TestLands(t *testing.T) {
	for _, tc := range []struct {
		outcomes []string
		want     bool
	}{
		{nil, false},
		{[]string{"stopped", "blocked"}, false},
		{[]string{"done"}, false}, // finished, but its code isn't on main yet
		{[]string{"stopped", "merged"}, true},
	} {
		var trs []Trial
		for _, o := range tc.outcomes {
			trs = append(trs, Trial{Outcome: o})
		}
		if got := Lands(trs); got != tc.want {
			t.Errorf("Lands(%v) = %v, want %v", tc.outcomes, got, tc.want)
		}
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
			{Task: "5", Model: "codex:", Project: "trials", Title: "wc on codex/default (wc)", Outcome: "merged"},
		},
	}
	got := TrialsOf(l, tasks)
	if n := len(got[Cell{Pick{"open", "a"}, "wc"}]); n != 2 {
		t.Errorf("open:a wc has %d trials, want 2", n)
	}
	if n := len(got[Cell{Pick{"codex", "gpt-5"}, "lru"}]); n != 1 {
		t.Errorf("codex:gpt-5 lru has %d trials, want 1", n)
	}
	// "codex:" is codex on its login's default model: a routable cell,
	// whose code is in trials/codex/default/.
	c := Cell{Pick{"codex", ""}, "wc"}
	if n := len(got[c]); n != 1 {
		t.Errorf("codex: wc has %d trials, want 1", n)
	}
	if want := filepath.Join("trials", "codex", "default", "wc"); c.Dir() != want {
		t.Errorf("codex: wc Dir = %q, want %q", c.Dir(), want)
	}
	if !slices.Contains(Picks(nil), c.Pick) {
		t.Errorf("Picks = %v, want codex: among them", Picks(nil))
	}
	if len(got) != 3 {
		t.Errorf("cells with trials = %v, want 3 (another project's trial doesn't count)", got)
	}
}

func TestTaskOf(t *testing.T) {
	for _, tc := range []struct {
		title string
		dir   string
	}{
		{"A tiny Go module: a word counter with tests", "wc"},
		{"Again (wc)", "wc"},
		{"An LRU cache with TTL", "lru"},
		{"wc on open/deepseek-v4-pro", "wc"},
		{"csvsum on open/gpt-oss-120b", "csvsum"},
		{"lru on codex/gpt-5", "lru"},
		{"WC on open/DeepSeek-v4-pro", "wc"}, // case insensitive
		{"something else", ""},
	} {
		t.Run(tc.title, func(t *testing.T) {
			if got := TaskOf(tc.title, tasks); got != tc.dir {
				t.Errorf("TaskOf(%q) = %q, want %q", tc.title, got, tc.dir)
			}
		})
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

// A hidden acceptance test (a _test.go.gz) is judged as a plain one is.
func TestJudgeHidden(t *testing.T) {
	c := Cell{Pick{"open", "m"}, "wc"}
	for _, tc := range []struct {
		name string
		code string
		pass bool
	}{
		{"passes", "package wc\n\nfunc Count(s string) int { return len(s) }\n", true},
		{"fails", "package wc\n\nfunc Count(s string) int { return 0 }\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := trial(t, c, map[string]string{"wc.go": tc.code})
			plain := filepath.Join(root, "tasks/wc/wc_test.go")
			b, err := os.ReadFile(plain)
			if err != nil {
				t.Fatal(err)
			}
			var z bytes.Buffer
			w := gzip.NewWriter(&z)
			w.Write(b)
			w.Close()
			os.Remove(plain)
			if err := os.WriteFile(plain+".gz", z.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			v, err := Judge(root, c)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Ran || v.Pass != tc.pass {
				t.Errorf("verdict = ran %v pass %v, want pass %v\n%s", v.Ran, v.Pass, tc.pass, v.Out)
			}
		})
	}
}

// A sealed trial (its files zipped by Seal) is judged as its plain code is,
// and sealing it again changes nothing.
func TestJudgeSealed(t *testing.T) {
	c := Cell{Pick{"open", "m"}, "wc"}
	for _, tc := range []struct {
		name string
		code string
		pass bool
	}{
		{"passes", "package wc\n\nfunc Count(s string) int { return len(s) }\n", true},
		{"fails", "package wc\n\nfunc Count(s string) int { return 0 }\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := trial(t, c, map[string]string{"wc.go": tc.code, "wc_test.go": "package wc\n"})
			dir := filepath.Join(root, c.Dir())
			if err := Seal(dir); err != nil {
				t.Fatal(err)
			}
			if plain, _ := filepath.Glob(filepath.Join(dir, "*.go")); len(plain) > 0 {
				t.Errorf("plain code left after Seal: %v", plain)
			}
			first, _ := os.ReadFile(filepath.Join(dir, "wc.go.gz"))
			if err := Seal(dir); err != nil {
				t.Fatal(err)
			}
			if again, _ := os.ReadFile(filepath.Join(dir, "wc.go.gz")); !bytes.Equal(first, again) {
				t.Errorf("sealing twice changed wc.go.gz")
			}
			v, err := Judge(root, c)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Ran || v.Pass != tc.pass {
				t.Errorf("verdict = ran %v pass %v, want pass %v\n%s", v.Ran, v.Pass, tc.pass, v.Out)
			}
		})
	}
}

// No landed trial's code sits plain on main: each is sealed, so a trial
// can't read or grep another trial's answer to its task in its checkout.
// A folder whose trial is still in flight (no trial of its cell has merged
// in ledger.json) may be plain: its own PR, and main between its merge and
// the seal workflow (.github/workflows/seal.yml) sealing it.
func TestTrialsSealed(t *testing.T) {
	raw, err := os.ReadFile("../../ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var l Ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		t.Fatal(err)
	}
	tasks, err := Tasks("../../tasks", "../../TASKS.md")
	if err != nil {
		t.Fatal(err)
	}
	trials := TrialsOf(l, tasks)
	for _, dir := range Plain("../..") {
		if c, ok := CellOf(dir); ok && Lands(trials[c]) {
			t.Errorf("%s holds plain code another trial could copy, and its trial has landed: seal it with `go run ./cmd/score -seal %s` (.github/workflows/seal.yml does on main) and commit the .go.gz files", dir, dir)
		}
	}
}

func TestCellOf(t *testing.T) {
	for _, tc := range []struct {
		dir  string
		want Cell
		ok   bool
	}{
		{"trials/open/laguna-free/wc", Cell{Pick{"open", "laguna-free"}, "wc"}, true},
		{"trials/opencode/opencode/big-pickle/gocache", Cell{Pick{"opencode", "opencode/big-pickle"}, "gocache"}, true},
		{"trials/open/wc", Cell{}, false},
		{"tasks/wc/x/y", Cell{}, false},
	} {
		if got, ok := CellOf(tc.dir); got != tc.want || ok != tc.ok {
			t.Errorf("CellOf(%q) = %v %v, want %v %v", tc.dir, got, ok, tc.want, tc.ok)
		}
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
		tests, err := Acceptance(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(tests) == 0 {
			t.Errorf("tasks/%s has no acceptance tests", k.Dir)
		}
		// Every task's tests are hidden, so a trial can't run them until
		// they're green and a pick can fail on how it reads the spec.
		if plain, _ := filepath.Glob(filepath.Join(dir, "*_test.go")); len(plain) > 0 {
			t.Errorf("tasks/%s has acceptance tests a trial can read: %v; gzip them (a _test.go.gz)", k.Dir, plain)
		}
		for f, b := range tests {
			if !strings.HasPrefix(string(b), "//go:build acceptance\n") {
				t.Errorf("tasks/%s/%s doesn't start with //go:build acceptance", k.Dir, f)
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

	// Save keeps demoted and routes (for the verbose summary), drops the rest
	in2 := `{"agents": ["x"], "models": {"openrouter": "openrouter at somewhere"}, "trials": [
		{"model": "openrouter", "project": "trials", "title": "wc", "outcome": "merged", "task": "1"}],
		"demoted": ["open:openrouter on deck goes after the rest of its worker route: its last 2 worker tasks here ended blocked without a merge"],
		"routes": {"worker": ["open:openrouter", "claude:opus"]},
		"trials_running": ["something"],
		"trials_not_counted": [],
		"unavailable": {"foo": "bar"}}`
	path2 := filepath.Join(t.TempDir(), "ledger2.json")
	if err := Save(strings.NewReader(in2), path2); err != nil {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(path2)
	s2 := string(b2)
	// demoted and routes survive
	if !strings.Contains(s2, "demoted") || !strings.Contains(s2, "open:openrouter on deck") {
		t.Errorf("saved ledger lost demoted:\n%s", s2)
	}
	if !strings.Contains(s2, "routes") || !strings.Contains(s2, "worker") || !strings.Contains(s2, "open:openrouter") {
		t.Errorf("saved ledger lost routes:\n%s", s2)
	}
	// agents, trials_running, trials_not_counted, unavailable are dropped
	for _, dropped := range []string{"agents", "trials_running", "trials_not_counted", "unavailable"} {
		if strings.Contains(s2, dropped) {
			t.Errorf("saved ledger kept %s:\n%s", dropped, s2)
		}
	}
}

func TestWorkerRecord(t *testing.T) {
	l := Ledger{
		Models: map[string]string{"openrouter": "nvidia/nemotron-3-ultra-550b-a55b:free at https://openrouter.ai/api/v1"},
		Trials: []Trial{
			{Task: "1", Model: "openrouter", Project: "trials", Title: "wc", Outcome: "merged"},
			{Task: "2", Model: "openrouter", Project: "trials", Title: "csvsum", Outcome: "merged"},
			{Task: "3", Model: "openrouter", Project: "trials", Title: "lru", Outcome: "merged"},
			{Task: "4", Model: "openrouter", Project: "trials", Title: "gocache", Outcome: "merged"},
			{Task: "5", Model: "openrouter", Project: "deck", Title: "some task", Outcome: "blocked"},
			{Task: "6", Model: "openrouter", Project: "deck", Title: "some task", Outcome: "blocked"},
		},
		Demoted: []string{"open:openrouter on deck goes after the rest of its worker route: its last 2 worker tasks here ended blocked without a merge"},
		Routes:  map[string][]string{"worker": {"open:openrouter", "claude:opus"}},
	}
	p := Pick{"open", "openrouter"}
	wr := workerRecord(l, p)
	if wr.TrialsTotal != 6 {
		t.Errorf("TrialsTotal = %d, want 6", wr.TrialsTotal)
	}
	if wr.TrialsMerged != 4 {
		t.Errorf("TrialsMerged = %d, want 4", wr.TrialsMerged)
	}
	if wr.TrialsBlocked != 2 {
		t.Errorf("TrialsBlocked = %d, want 2", wr.TrialsBlocked)
	}
	if !wr.Demoted {
		t.Errorf("Demoted = false, want true")
	}
	if wr.DeckPosition != 1 {
		t.Errorf("DeckPosition = %d, want 1", wr.DeckPosition)
	}

	// Test a pick with failing trial verdicts
	l2 := Ledger{
		Models: map[string]string{"modelx": "modelx at somewhere"},
		Trials: []Trial{
			{Task: "1", Model: "modelx", Project: "trials", Title: "wc", Outcome: "merged"},
		},
	}
	p2 := Pick{"open", "modelx"}
	wr2 := workerRecord(l2, p2)
	if wr2.TrialsTotal != 1 {
		t.Errorf("TrialsTotal = %d, want 1", wr2.TrialsTotal)
	}
	if wr2.TrialsMerged != 1 {
		t.Errorf("TrialsMerged = %d, want 1", wr2.TrialsMerged)
	}
	if wr2.Demoted {
		t.Errorf("Demoted = true, want false")
	}
	if wr2.DeckPosition != 0 {
		t.Errorf("DeckPosition = %d, want 0", wr2.DeckPosition)
	}

	// A demoted pick whose name starts with another's doesn't demote the shorter one
	l3 := Ledger{
		Models:  map[string]string{"openrouter": "x at y", "openrouter-big": "z at y"},
		Demoted: []string{"open:openrouter-big on deck goes after the rest of its worker route: ..."},
	}
	if workerRecord(l3, Pick{"open", "openrouter"}).Demoted {
		t.Errorf("open:openrouter demoted by open:openrouter-big's line")
	}
	if !workerRecord(l3, Pick{"open", "openrouter-big"}).Demoted {
		t.Errorf("open:openrouter-big not demoted")
	}
}

// A bug-fix task's starting code (tasks/<task>/_start, which go ignores) is
// what a trial copies and fixes: it builds and fails the acceptance tests,
// so landing it unchanged is a fail, not a pass.
func TestStartsFail(t *testing.T) {
	got, err := Tasks("../../tasks", "../../TASKS.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range got {
		start := filepath.Join("../../tasks", k.Dir, "_start")
		code, err := goFiles(start, false)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || len(code) == 0 {
			t.Fatalf("%s: %v %v", start, code, err)
		}
		c := Cell{Pick{"start", "start"}, k.Dir}
		root := t.TempDir()
		for _, dir := range []string{c.Dir(), filepath.Join("tasks", k.Dir)} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		tests, _ := os.ReadDir(filepath.Join("../../tasks", k.Dir))
		for _, f := range code {
			copyFile(t, f, filepath.Join(root, c.Dir(), filepath.Base(f)))
		}
		for _, e := range tests {
			if !e.IsDir() {
				copyFile(t, filepath.Join("../../tasks", k.Dir, e.Name()), filepath.Join(root, "tasks", k.Dir, e.Name()))
			}
		}
		v, err := Judge(root, c)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Ran || v.Pass {
			t.Errorf("tasks/%s/_start: verdict ran %v pass %v, want a fail\n%s", k.Dir, v.Ran, v.Pass, v.Out)
		} else if strings.Contains(v.Out, "build failed") || strings.Contains(v.Out, "[setup failed]") {
			t.Errorf("tasks/%s/_start doesn't build against its tests:\n%s", k.Dir, v.Out)
		}
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFault(t *testing.T) {
	for _, tc := range []struct {
		trial Trial
		want  bool
	}{
		{Trial{Outcome: "stopped", Reason: "closed by the lead: this trial's files match main's; its account says it copied the reference code"}, true},
		{Trial{Outcome: "stopped", Reason: "left open: the warden: the PR's ledger.json hunk is a self-written entry"}, true},
		{Trial{Outcome: "stopped", Reason: "left open"}, false},
		{Trial{Outcome: "stopped", Reason: "closed by the lead: Trial worker went quiet and never finished. Closing to free its slot."}, false},
		{Trial{Outcome: "stopped", Reason: "withdrawn by lead-x: misfiled, copied the wrong model into ledger.json"}, false}, // a Hive fault
		{Trial{Outcome: "blocked", Reason: "stopped while quiet by lead-x: no API key stored. Not a model result."}, false},
		{Trial{Outcome: "merged", Reason: "closed by the lead: copied"}, false},
	} {
		if _, got := Fault([]Trial{tc.trial}); got != tc.want {
			t.Errorf("Fault(%q %q) = %v, want %v", tc.trial.Outcome, tc.trial.Reason, got, tc.want)
		}
	}
}

func TestVerdicts(t *testing.T) {
	landed := Cell{Pick{"open", "a"}, "wc"}
	faulted := Cell{Pick{"open", "a"}, "lru"}
	both := Cell{Pick{"open", "b"}, "wc"}
	none := Cell{Pick{"open", "b"}, "lru"}
	copied := Trial{Outcome: "stopped", Reason: "closed by the lead: it copied another trial's code"}
	trials := map[Cell][]Trial{
		landed:  {{Outcome: "merged"}},
		faulted: {copied},
		both:    {copied, {Outcome: "merged"}},
		none:    {{Outcome: "stopped", Reason: "withdrawn by lead-x: the key allows only :free models"}},
	}
	verdict := Verdicts(trials, func(Cell) Verdict { return Verdict{Ran: true, Pass: true} })
	for _, tc := range []struct {
		c             Cell
		ok, ran, pass bool
	}{
		{landed, true, true, true},
		{faulted, true, true, false}, // the pick's own fault fails
		{both, true, true, true},     // a later trial's landed code is judged
		{none, false, false, false},  // a Hive fault gives no verdict
	} {
		v, ok := verdict(tc.c)
		if ok != tc.ok || v.Ran != tc.ran || v.Pass != tc.pass {
			t.Errorf("%v: verdict %+v %v, want ran=%v pass=%v %v", tc.c, v, ok, tc.ran, tc.pass, tc.ok)
		}
	}
}
