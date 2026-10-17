package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var realGhPath = sync.OnceValue(func() string {
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, "gh")
		if _, err := exec.LookPath(p); err != nil {
			continue
		}
		// Skip a gh that is a symlink to ghauto itself.
		if resolved, err := filepath.EvalSymlinks(p); err == nil && resolved == self {
			continue
		}
		return p
	}
	return ""
})

// realGh returns the path of the real gh binary, skipping ghauto when it is
// installed as a "gh" shim on PATH.
func realGh() string {
	if p := realGhPath(); p != "" {
		return p
	}
	return "gh"
}
