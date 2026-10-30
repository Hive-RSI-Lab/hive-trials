// Package config reads a service's settings from INI-like files:
//
//	# a comment
//	name = api
//	[db]
//	host = "db.internal"   # db.host
//	port = 5432            # db.port
//
// A key in a [section] is named section.key; a key set twice keeps the
// value set last.
package config

import (
	"io"
	"os"
	"sort"
)

// Config is the settings a file set.
type Config struct {
	values map[string]string
}

// Get is key's value and whether it was set.
func (c *Config) Get(key string) (string, bool) {
	v, ok := c.values[key]
	return v, ok
}

// Keys are the keys set, sorted.
func (c *Config) Keys() []string {
	out := make([]string, 0, len(c.values))
	for k := range c.values {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Load reads the config file at path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f, path)
}

// Parse reads a config file from r; name is what its errors call it.
func Parse(r io.Reader, name string) (*Config, error) {
	c := &Config{values: map[string]string{}}
	if err := parse(r, name, c); err != nil {
		return nil, err
	}
	return c, nil
}
