// Command score prints the share of routable model × harness × task cells
// that have a verdict: one number between 0 and 1, higher is better.
//
// A cell counts only when a ledger trial on its pick and task landed its
// code (its outcome is merged), and then its verdict is whether that code,
// in trials/<harness>/<model>/<task>/ on main, passes the task's acceptance
// tests in tasks/<task>/. The score runs them itself, so a trial that
// changed or dropped its own tests is judged the same. A trial that ended
// on a Hive fault (a 403, a lead closing it, the warden leaving it open)
// landed nothing and gives no verdict; nor does code in a cell's folder
// that no trial of that cell landed (put there by hand, or by a trial on
// another pick or task).
//
// The ledger is ledger.json, a snapshot of `hive agent models`, saved with
//
//	hive agent models | go run ./cmd/score -save
//
// because the score check's sandbox can't call it. -v also lists how each
// cell's trials went: outcome, reason, cost, minutes and turns.
//
// A trial lands its code sealed, each file zipped by
//
//	go run ./cmd/score -seal trials/<harness>/<model>/<task>
//
// so a later trial of the same task can't read it in its checkout.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Native are the picks a harness runs on its own models, beside the named
// models (which only the open harness runs: claude runs Anthropic models,
// codex OpenAI's, cursor and opencode their own). Claude isn't here: the
// Hive already knows it from all its other work.
var Native = []Pick{
	{"codex", "gpt-5"},
	{"cursor", "auto"},
	{"opencode", "opencode/big-pickle"},
}

// Project is the project whose trials count.
const Project = "trials"

// Trial is one record of the ledger's trials.
type Trial struct {
	Task         string  `json:"task"`
	Model        string  `json:"model"`
	Project      string  `json:"project"`
	Title        string  `json:"title"`
	Outcome      string  `json:"outcome"`
	Reason       string  `json:"reason,omitempty"`
	Cost         float64 `json:"cost"`
	Minutes      int     `json:"minutes"`
	Turns        int     `json:"turns"`
	ReviewRounds int     `json:"review_rounds"`
}

// Ledger is what the score reads from `hive agent models`: the named models
// and the trials, plus the worker record fields (demoted, etc.).
type Ledger struct {
	Models  map[string]string   `json:"models"`
	Trials  []Trial             `json:"trials"`
	Demoted []string            `json:"demoted,omitempty"`
	Routes  map[string][]string `json:"routes,omitempty"`
}

// Task is one task of the fixed set: its folder under tasks/ and its name in
// TASKS.md.
type Task struct{ Dir, Name string }

// Pick is one harness and model, as a route names it.
type Pick struct{ Harness, Model string }

func (p Pick) String() string { return p.Harness + ":" + p.Model }

// Cell is one pick on one task.
type Cell struct {
	Pick
	Task string
}

// Dir is where the cell's trial puts its code.
func (c Cell) Dir() string { return filepath.Join("trials", c.Harness, c.Model, c.Task) }

// Verdict is how a cell's code did on its acceptance tests; Ran is false
// when there is no code to test.
type Verdict struct {
	Ran, Pass bool
	Out       string
}

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

// Picks are the routable picks: every named model on the open harness, then
// Native.
func Picks(named map[string]string) []Pick {
	var out []Pick
	for m := range named {
		out = append(out, Pick{"open", m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return append(out, Native...)
}

// PickOf reads a trial's model as the ledger names it: a named model runs on
// the open harness, "harness:model" on that harness.
func PickOf(model string, named map[string]string) Pick {
	if _, ok := named[model]; ok {
		return Pick{"open", model}
	}
	if h, m, ok := strings.Cut(model, ":"); ok {
		return Pick{h, m}
	}
	return Pick{"", model}
}

// TaskOf is the task a trial's title names: the task's name from TASKS.md
// (say "Word Counter"), its folder in parentheses (say "(wc)"), or a title
// that starts with "<dir> on " (say "wc on open/deepseek-v4-pro"); "" when
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
		// Also match the "<dir> on <harness>/<model>" pattern
		if strings.HasPrefix(t, strings.ToLower(k.Dir)+" on ") {
			return k.Dir
		}
	}
	return ""
}

// Cells are the routable cells, in order.
func Cells(l Ledger, tasks []Task) []Cell {
	var out []Cell
	for _, p := range Picks(l.Models) {
		for _, t := range tasks {
			out = append(out, Cell{p, t.Dir})
		}
	}
	return out
}

// Landed is the outcome of a trial whose code is on main.
const Landed = "merged"

// Lands says whether any of a cell's trials landed its code.
func Lands(trials []Trial) bool {
	for _, t := range trials {
		if t.Outcome == Landed {
			return true
		}
	}
	return false
}

// Score is the share of cells that a trial landed (by landed) and whose
// code ran its acceptance tests (by judge); a cell no trial landed isn't
// judged.
func Score(cells []Cell, landed func(Cell) bool, judge func(Cell) Verdict) (float64, map[Cell]Verdict) {
	vs := map[Cell]Verdict{}
	n := 0
	for _, c := range cells {
		if !landed(c) {
			continue
		}
		vs[c] = judge(c)
		if vs[c].Ran {
			n++
		}
	}
	if len(cells) == 0 {
		return 0, vs
	}
	return float64(n) / float64(len(cells)), vs
}

// Judge runs the acceptance tests in root/tasks/<task> against the cell's
// code in root/<c.Dir()>: its non-test Go files, sealed or plain (see
// Code), in a module of its own, so the trial's own tests play no part. The
// acceptance tests build only with the acceptance tag (tasks/ holds no
// answer for them to build against on main), so Judge sets it. A cell with
// no code there didn't run. An error is the score's own fault (no go, no
// temp dir, a sealed file that won't unzip), not the trial's.
func Judge(root string, c Cell) (Verdict, error) {
	code, err := Code(filepath.Join(root, c.Dir()))
	if os.IsNotExist(err) || len(code) == 0 {
		return Verdict{}, nil
	}
	if err != nil {
		return Verdict{}, err
	}
	tests, err := Acceptance(filepath.Join(root, "tasks", c.Task))
	if err != nil {
		return Verdict{}, err
	}
	dir, err := os.MkdirTemp("", "trial-")
	if err != nil {
		return Verdict{}, err
	}
	defer os.RemoveAll(dir)
	for name, b := range code {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return Verdict{}, err
		}
	}
	for name, b := range tests {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return Verdict{}, err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module trial\n\ngo 1.22\n"), 0o644); err != nil {
		return Verdict{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-tags", "acceptance", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return Verdict{Ran: true, Pass: true}, nil
	case errors.As(err, &exit): // failed tests, a build error or the timeout: the code's
		return Verdict{Ran: true, Out: strings.ReplaceAll(out.String(), dir+"/", "")}, nil
	default:
		return Verdict{}, err
	}
}

// Acceptance are a task folder's acceptance tests by file name: its
// _test.go files, and its hidden ones, each a _test.go.gz that the score
// unzips, so a trial that only reads TASKS.md and the starting code can't
// read or grep them (a trial that unzips one has read the answer key).
func Acceptance(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(n, "_test.go"):
			b, err := os.ReadFile(filepath.Join(dir, n))
			if err != nil {
				return nil, err
			}
			out[n] = b
		case strings.HasSuffix(n, "_test.go.gz"):
			b, err := gunzip(filepath.Join(dir, n))
			if err != nil {
				return nil, err
			}
			out[strings.TrimSuffix(n, ".gz")] = b
		}
	}
	return out, nil
}

// Code is a trial folder's code by file name: its non-test .go files, and
// its sealed ones (a .go.gz, see Seal), unzipped.
func Code(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasSuffix(n, "_test.go") || strings.HasSuffix(n, "_test.go.gz") {
			continue
		}
		var b []byte
		switch {
		case strings.HasSuffix(n, ".go"):
			b, err = os.ReadFile(filepath.Join(dir, n))
		case strings.HasSuffix(n, ".go.gz"):
			b, err = gunzip(filepath.Join(dir, n))
			n = strings.TrimSuffix(n, ".gz")
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		out[n] = b
	}
	return out, nil
}

// Seal zips each .go file in a trial folder (its tests too) into a .go.gz
// beside it and removes the plain one, so the next trial of the same task
// can't read or grep this one's answer in its checkout; the score unzips
// them to judge. The zip carries no name or time, so sealing the same code
// twice gives the same bytes.
func Seal(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") {
			continue
		}
		path := filepath.Join(dir, n)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var z bytes.Buffer
		w, _ := gzip.NewWriterLevel(&z, gzip.BestCompression)
		w.Write(b)
		if err := w.Close(); err != nil {
			return err
		}
		if err := os.WriteFile(path+".gz", z.Bytes(), 0o644); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// gunzip is the unzipped content of the file at path.
func gunzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	b, err := io.ReadAll(z)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return b, nil
}

// goFiles are dir's .go files: the tests, or the rest.
func goFiles(dir string, tests bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".go") && strings.HasSuffix(n, "_test.go") == tests {
			out = append(out, filepath.Join(dir, n))
		}
	}
	return out, nil
}

// TrialsOf are this project's ledger trials of each cell.
func TrialsOf(l Ledger, tasks []Task) map[Cell][]Trial {
	out := map[Cell][]Trial{}
	for _, tr := range l.Trials {
		if tr.Project != Project {
			continue
		}
		c := Cell{PickOf(tr.Model, l.Models), TaskOf(tr.Title, tasks)}
		out[c] = append(out[c], tr)
	}
	return out
}

// Save writes the models and this project's trials from `hive agent models`
// output in r to path. It keeps demoted and routes for the verbose summary.
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

// line is s's first line, at most n runes.
func line(s string, n int) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// WorkerRecord is a pick's standing from hive agent models across all projects.
type WorkerRecord struct {
	Pick          Pick
	TrialsTotal   int
	TrialsMerged  int
	TrialsStopped int
	TrialsBlocked int
	TrialsDone    int
	Demoted       bool
	DeckPosition  int // 1-based index in the deck, 0 = not in deck
}

// workerRecord is p's worker record in l: its trials by outcome, whether
// it is demoted and where it sits on the worker route.
func workerRecord(l Ledger, p Pick) WorkerRecord {
	wr := WorkerRecord{Pick: p}
	for _, tr := range l.Trials {
		// PickOf handles both named models and harness:model format
		tp := PickOf(tr.Model, l.Models)
		if tp.Harness == p.Harness && tp.Model == p.Model {
			wr.TrialsTotal++
			switch tr.Outcome {
			case "merged":
				wr.TrialsMerged++
			case "stopped":
				wr.TrialsStopped++
			case "blocked":
				wr.TrialsBlocked++
			case "done":
				wr.TrialsDone++
			}
		}
	}
	// Demoted if a demoted line names exactly this pick as its first word:
	// "open:openrouter on deck goes after the rest of its worker route: ..."
	for _, d := range l.Demoted {
		if name, _, _ := strings.Cut(d, " "); name == p.String() {
			wr.Demoted = true
			break
		}
	}
	// Check deck position (from routes.worker)
	if routes, ok := l.Routes["worker"]; ok {
		for i, r := range routes {
			if r == p.String() {
				wr.DeckPosition = i + 1
				break
			}
		}
	}
	return wr
}

// why is the line of go test's output that says why it failed: the first
// failing test or compile error, else its first line.
func why(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "--- FAIL") || strings.Contains(l, ".go:") {
			return strings.TrimSpace(l)
		}
	}
	return out
}

func main() {
	save := flag.Bool("save", false, "save `hive agent models` output on stdin as the ledger")
	verbose := flag.Bool("v", false, "list every cell: its verdict and how its trials went")
	ledger := flag.String("ledger", "ledger.json", "the ledger snapshot")
	seal := flag.Bool("seal", false, "seal the trial folders named as arguments (see Seal) and exit")
	flag.Parse()
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, "score:", err)
		os.Exit(1)
	}
	if *seal {
		if flag.NArg() == 0 {
			fail(errors.New("-seal: name the trial folders to seal, say trials/open/<model>/<task>"))
		}
		for _, dir := range flag.Args() {
			if err := Seal(dir); err != nil {
				fail(err)
			}
		}
		return
	}
	if *save {
		if err := Save(os.Stdin, *ledger); err != nil {
			fail(err)
		}
	}
	raw, err := os.ReadFile(*ledger)
	if err != nil {
		fail(err)
	}
	var l Ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		fail(fmt.Errorf("%s: %w", *ledger, err))
	}
	tasks, err := Tasks("tasks", "TASKS.md")
	if err != nil {
		fail(err)
	}
	cells := Cells(l, tasks)
	trials := TrialsOf(l, tasks)
	s, vs := Score(cells, func(c Cell) bool { return Lands(trials[c]) }, func(c Cell) Verdict {
		v, err := Judge(".", c)
		if err != nil {
			fail(fmt.Errorf("judging %s %s: %w", c.Pick, c.Task, err))
		}
		return v
	})
	if *verbose {
		for _, c := range cells {
			mark := "    "
			if v := vs[c]; v.Ran && v.Pass {
				mark = "pass"
			} else if v.Ran {
				mark = "fail"
			}
			fmt.Printf("[%s] %s %s\n", mark, c.Pick, c.Task)
			if v := vs[c]; v.Ran && !v.Pass {
				fmt.Printf("       %s\n", line(why(v.Out), 120))
			}
			if !Lands(trials[c]) {
				if code, _ := Code(c.Dir()); len(code) > 0 {
					fmt.Printf("       code in %s, but no trial of this cell landed it: not counted\n", c.Dir())
				}
			}
			for _, t := range trials[c] {
				fmt.Printf("       %s %s: %d min, %d turns, $%.2f: %s\n", t.Task, t.Outcome, t.Minutes, t.Turns, t.Cost, line(t.Reason, 100))
			}
		}

		// Per-pick summary: trial verdicts vs worker record (from hive agent models)
		fmt.Println()
		fmt.Println("Per-pick trial verdicts vs worker record:")
		picks := Picks(l.Models)
		for _, p := range picks {
			wr := workerRecord(l, p)
			// Count trial verdicts for this pick
			trialPass := 0
			trialFail := 0
			trialNoCode := 0
			trialMergedCells := 0
			for _, c := range cells {
				if c.Pick == p && Lands(trials[c]) {
					trialMergedCells++
					v := vs[c]
					if v.Ran {
						if v.Pass {
							trialPass++
						} else {
							trialFail++
						}
					} else {
						trialNoCode++
					}
				}
			}
			if trialMergedCells == 0 && wr.TrialsTotal == 0 {
				fmt.Printf("  %s: no data\n", p)
				continue
			}
			fmt.Printf("  %s: trials=%d(merged=%d stopped=%d blocked=%d done=%d) demoted=%v deck=%d\n",
				p, wr.TrialsTotal, wr.TrialsMerged, wr.TrialsStopped, wr.TrialsBlocked, wr.TrialsDone, wr.Demoted, wr.DeckPosition)
			if trialMergedCells > 0 {
				fmt.Printf("       trial verdicts: %d merged cells, %d pass, %d fail, %d no-code\n",
					trialMergedCells, trialPass, trialFail, trialNoCode)
				if trialFail > 0 {
					fmt.Printf("       ^^^ %d merged cell(s) failed acceptance tests\n", trialFail)
				} else if trialPass > 0 {
					fmt.Printf("       all %d merged cell(s) pass\n", trialPass)
				}
			}
			// Flag the gap: trials say pass but worker record shows demoted/blocked
			if wr.Demoted && trialFail == 0 && trialPass > 0 {
				fmt.Printf("       *** GAP: trials pass but demoted on deck\n")
			}
			if wr.TrialsBlocked > 0 && trialFail == 0 && trialPass > 0 {
				fmt.Printf("       *** GAP: trials pass but %d blocked trial(s) in worker record\n", wr.TrialsBlocked)
			}
		}
	}
	fmt.Printf("%.4f\n", s)
}
