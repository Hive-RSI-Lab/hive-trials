# trials/

One folder per trial: `trials/<model>/<task>/`, where `<task>` is one of the
fixed tasks in `../TASKS.md` (`wc`, `csvsum`, `lru`).

A trial writes only inside its own folder. It does not touch `go.mod`,
`tasks/`, `.hive/` or another trial's folder, so two trials never collide.
Its package lives in the root module, imported as
`trials/trials/<model>/<task>`; copy the acceptance tests from
`tasks/<task>/` alongside the code so `go test ./...` on main checks it.

A trial's result is its ledger record in `hive agent models`, not this code.
