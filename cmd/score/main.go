// Command score prints the share of model × harness × task cells that have a
// trial verdict in the ledger: one number between 0 and 1, higher is better.
//
// The score check runs in a sandbox that can't call `hive agent models`, so
// the ledger it reads is ledger.json, a snapshot of that op's output saved
// with
//
//	hive agent models | go run ./cmd/score -save
//
// Run with no flags, it reads ledger.json, tasks/ and TASKS.md and prints the
// score; -v also lists every cell.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Harnesses are the harnesses a model can run on.
var Harnesses = []string{"claude", "codex", "open", "cursor", "opencode"}

// Project is the project whose trials count.
const Project = "trials"

// Trial is one record of the ledger's trials.
type Trial struct {
	Task    string `json:"task"`
	Model   string `json:"model"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
}

// Ledger is what the score reads from `hive agent models`: the named models
// and the trials.
type Ledger struct {
	Models map[string]string `json:"models"`
	Trials []Trial           `json:"trials"`
}

// Task is one task of the fixed set: its folder under tasks/ and its name in
// TASKS.md.
type Task struct{ Dir, Name string }

// Cell is one model on one harness on one task.
type Cell struct{ Model, Harness, Task string }

var heading = regexp.MustCompile(`(?m)^##\s+\d+\.\s+(.+?)\s+\(([^)]+)\)\s*$`)

// Tasks is every folder under tasksDir, named by its "## N. Name (dir)"
// heading in tasksMD.
func Tasks(tasksDir, tasksMD string) ([]Task, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	if md, err := os.ReadFile(tasksMD); err == nil {
		for _, m := range heading.FindAllStringSubmatch(string(md), -1) {
			names[m[2]] = m[1]
		}
	}
	var out []Task
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, Task{Dir: e.Name(), Name: names[e.Name()]})
		}
	}
	return out, nil
}

// Pick reads a trial's model as the ledger names it: a named model runs on
// the open harness, "harness:model" on that harness.
func Pick(model string, named map[string]string) (harness, name string) {
	if _, ok := named[model]; ok {
		return "open", model
	}
	if h, m, ok := strings.Cut(model, ":"); ok {
		return h, m
	}
	return "", model
}

// TaskOf is the task a trial's title names: the task's name from TASKS.md
// (say "Word Counter") or its folder in parentheses (say "(wc)"); "" when
// none does.
func TaskOf(title string, tasks []Task) string {
	t := strings.ToLower(title)
	for _, k := range tasks {
		if k.Name != "" && strings.Contains(t, strings.ToLower(k.Name)) {
			return k.Dir
		}
		if strings.Contains(t, "("+strings.ToLower(k.Dir)+")") {
			return k.Dir
		}
	}
	return ""
}

// Score is the share of cells with a verdict, and every cell with whether it
// has one.
func Score(l Ledger, tasks []Task) (float64, map[Cell]bool) {
	cells := map[Cell]bool{}
	for m := range l.Models {
		for _, h := range Harnesses {
			for _, t := range tasks {
				cells[Cell{m, h, t.Dir}] = false
			}
		}
	}
	for _, tr := range l.Trials {
		if tr.Project != Project || tr.Outcome == "" {
			continue
		}
		h, m := Pick(tr.Model, l.Models)
		c := Cell{m, h, TaskOf(tr.Title, tasks)}
		if _, ok := cells[c]; ok {
			cells[c] = true
		}
	}
	if len(cells) == 0 {
		return 0, cells
	}
	n := 0
	for _, ok := range cells {
		if ok {
			n++
		}
	}
	return float64(n) / float64(len(cells)), cells
}

// Save writes the models and this project's trials from `hive agent models`
// output in r to path.
func Save(r io.Reader, path string) error {
	var l Ledger
	if err := json.NewDecoder(r).Decode(&l); err != nil {
		return fmt.Errorf("reading hive agent models output: %w", err)
	}
	if l.Models == nil {
		return fmt.Errorf("no models in the input: is it `hive agent models` output?")
	}
	kept := []Trial{}
	for _, t := range l.Trials {
		if t.Project == Project {
			kept = append(kept, t)
		}
	}
	l.Trials = kept
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func main() {
	save := flag.Bool("save", false, "save `hive agent models` output on stdin as the ledger")
	verbose := flag.Bool("v", false, "list every cell")
	ledger := flag.String("ledger", "ledger.json", "the ledger snapshot")
	flag.Parse()
	if *save {
		if err := Save(os.Stdin, *ledger); err != nil {
			fmt.Fprintln(os.Stderr, "score:", err)
			os.Exit(1)
		}
	}
	raw, err := os.ReadFile(*ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "score:", err)
		os.Exit(1)
	}
	var l Ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		fmt.Fprintln(os.Stderr, "score:", *ledger+":", err)
		os.Exit(1)
	}
	tasks, err := Tasks("tasks", "TASKS.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, "score:", err)
		os.Exit(1)
	}
	s, cells := Score(l, tasks)
	if *verbose {
		var keys []Cell
		for c := range cells {
			keys = append(keys, c)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, b := keys[i], keys[j]
			if a.Model != b.Model {
				return a.Model < b.Model
			}
			if a.Harness != b.Harness {
				return a.Harness < b.Harness
			}
			return a.Task < b.Task
		})
		for _, c := range keys {
			mark := " "
			if cells[c] {
				mark = "x"
			}
			fmt.Printf("[%s] %s %s %s\n", mark, c.Model, c.Harness, c.Task)
		}
	}
	fmt.Printf("%.4f\n", s)
}
