# Lessons

What was tried and why it didn't stay. Read before working here.

## A trial never edits ledger.json

Trials kept writing their own entries in `ledger.json`, declaring their own outcomes as "merged" before they actually merged. This moved the score in favor of their work. The ledger is a snapshot made by `hive agent models | go run ./cmd/score -save` after a trial ends, not something a trial writes itself.

- PR #42 (opencode) and PR #49 (qwen3-coder-next) had their self-written ledger entries stripped before merge (commit c2b34a0)
- PR #50 (laguna-free) was stopped by the warden for the same fault
- The score counts a cell only when the ledger shows a trial of that pick and task merged

**Rule:** A trial writes only inside its own folder (`trials/<harness>/<model>/<task>/`). It does not touch `ledger.json`, `go.mod`, `tasks/`, `.hive/` or another trial's folder.