# trials/

One folder per trial: `trials/<harness>/<model>/<task>/`, where `<harness>`
and `<model>` are the trial's pick (a named model like `gpt-oss-120b` runs on
`open`) and `<task>` is one of the fixed tasks in `../TASKS.md` (`wc`,
`csvsum`, `lru`, `gocache`, `config`).

A trial writes only inside its own folder. It does not touch `go.mod`,
`tasks/`, `.hive/`, `ledger.json` or another trial's folder, so two trials never collide.
Its package is the one the task's tests name (`package lru`), in the root
module, so it must build: main's `go test ./...` builds it.

Land the attempt whether or not it passes: its verdict is whether it passes
the acceptance tests in `tasks/<task>/`, which `go run ./cmd/score` runs
against it. There is no reference answer in the repo to copy: write the
code from the task's description in `../TASKS.md` and its tests. A task on
existing code (`gocache`, `config`) starts from the code in
`../tasks/<task>/_start/`: copy it into your folder and change it there;
never edit `_start/`. A task's hidden tests (a `_test.go.gz`) are not yours
to open: work from its description. A failing
attempt is a verdict too; one that never lands is none. The cell counts only
once the ledger shows a trial of that pick and task merged, so code put in a
folder any other way doesn't count.

## Known limits

**OpenRouter free-model daily cap is shared Hive-wide** (recorded 2026-10-27)

- All free models on OpenRouter (e.g., `openrouter` = nemotron :free, `laguna-free`) share a single daily request cap per API key.
- The worker route puts `open:openrouter` first, so routed work consumes the same cap that trials need.
- On 2026-10-27 both free `wc` trials parked at 00:00Z waiting for the cap to reset, after hitting 429 `free-models-per-day-high-balance`.
- `inkling-free` is refused by OpenRouter outside the "agentic harnesses" listed at openrouter.ai/apps (403).

Route free models with this in mind: trials and routed work draw from the same daily quota.
