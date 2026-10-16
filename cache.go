package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// cache maps "host/owner/repo" to the gh login that should be used.
type cache map[string]string

func cachePath() string {
	if p := os.Getenv("GHAUTO_CACHE"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "ghauto", "cache.json")
}

func loadCache() cache {
	c := cache{}
	data, err := os.ReadFile(cachePath())
	if err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

// saveCache writes atomically so parallel git processes don't corrupt it.
func saveCache(c cache) {
	path := cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	tmp, err := os.CreateTemp(filepath.Dir(path), "cache-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	tmp.Close()
	if werr != nil {
		os.Remove(tmp.Name())
		return
	}
	os.Rename(tmp.Name(), path)
}
