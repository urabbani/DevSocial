package app

import (
	"os"
	"path/filepath"
)

func projectPath(parts ...string) string {
	relative := filepath.Join(parts...)
	for _, base := range candidateProjectRoots() {
		candidate := filepath.Join(base, relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return relative
}

func candidateProjectRoots() []string {
	seen := map[string]bool{}
	var roots []string

	addChain := func(start string) {
		if start == "" {
			return
		}
		dir := start
		for {
			if !seen[dir] {
				seen[dir] = true
				roots = append(roots, dir)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if wd, err := os.Getwd(); err == nil {
		addChain(wd)
	}
	if exe, err := os.Executable(); err == nil {
		addChain(filepath.Dir(exe))
	}

	return roots
}
