# trials

A scratch project for trying models and harnesses on small real tasks before routing real work to them.

## Word Counter (wc)

A tiny Go package that counts words in a string, lower-casing and splitting on non-letters.

### Usage

```go
import "trials/wc"

counts := wc.Count("Hello, WORLD! hello?")
// counts == map[string]int{"hello": 2, "world": 1}
```

### Running tests

```bash
go test ./wc/...
```