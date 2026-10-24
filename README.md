# trials

## LRU Cache with TTL

This project provides a concurrency‑safe LRU cache with per‑entry TTL.

### Running tests

```bash
go test ./...
```

The tests include eviction order, expiration, in‑place updates, and a concurrency test (run with `-race` to verify race‑free operation).

A scratch project for trying models and harnesses on small real tasks before routing real work to them.
