package gocache

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fileName is where an ID's file lives in dir, as cmd/go/internal/cache lays
// out GOCACHE: in the folder named by the ID's first byte, "<id>-a" for an
// action's entry and "<id>-d" for an output.
func fileName(dir string, id []byte, key string) string {
	return filepath.Join(dir, fmt.Sprintf("%02x", id[0]), fmt.Sprintf("%x", id)+"-"+key)
}

// entryLine is an action's entry as cmd/go/internal/cache writes it:
// "v1 <action> <output> <size> <time>\n", size and time padded to 20.
func entryLine(action, output []byte, size int64, at time.Time) []byte {
	return []byte(fmt.Sprintf("v1 %x %x %20d %20d\n", action, output, size, at.UnixNano()))
}

// parseEntry reads an entry line for action; ok is false when it is not
// one.
func parseEntry(raw, action []byte) (Entry, bool) {
	f := strings.Fields(string(raw))
	if len(f) != 5 || f[0] != "v1" || f[1] != hex.EncodeToString(action) || !strings.HasSuffix(string(raw), "\n") {
		return Entry{}, false
	}
	out, err := hex.DecodeString(f[2])
	size, err2 := strconv.ParseInt(f[3], 10, 64)
	ns, err3 := strconv.ParseInt(f[4], 10, 64)
	if err != nil || err2 != nil || err3 != nil || len(out) != 32 || size < 0 {
		return Entry{}, false
	}
	return Entry{Output: out, Size: size, At: time.Unix(0, ns)}, true
}
